package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/frizner/glsolr"

	"github.com/akamensky/argparse"
)

const (
	// the program name and version
	name    = "gosolrdump"
	version = "0.1"
	// datetime format
	timeFmt = "20060102-150405"

	// RE to check web link to a Solr collection
	reLink = `^((https|http):\/\/((([a-z0-9-.]+:\d+)|([a-z0-9-.]+))|([a-z0-9-]+))\/solr\/([\\._A-Za-z0-9\\-]+|([\\._A-Za-z0-9\\-]+)\/))$`

	// the names of environment variables to get the username and password if they aren't set in the command line
	userEnv  = "SOLRUSER"
	passwEnv = "SOLRPASSW"

	// default settings

	// http timeout in seconds by default
	dftHTTPTimeout = 180

	// Permissions to create the dump directory
	dftDirPerms = "0755"

	// Amount of rows in one query
	dftRows = 100000
)

type params struct {
	cLink, q, fl, sort, dstDir, user, passw string
	rows, httpTimeout                       int
	dirPerms                                os.FileMode
}

// Parse arguments
func parceArgs(name, lMask string, args []string) (colName string, p *params, err error) {
	parsHelp := fmt.Sprintf("%s dumps and saves documents from a Solr collection in json format", name)
	parser := argparse.NewParser(name, fmt.Sprintf("%s ", parsHelp))
	cLink := parser.String("c", "colllink", &argparse.Options{Required: true,
		Help: "http link to a Solr collection like http[s]://address[:port]/solr/collection"})

	q := parser.String("q", "query", &argparse.Options{Required: false, Default: "*:*",
		Help: "Q parameter"})

	fl := parser.String("f", "fieldlist", &argparse.Options{Required: false, Default: "",
		Help: "Fields list. All fields of documents are exported by default"})

	sort := parser.String("s", "sort", &argparse.Options{Required: true,
		Help: "Sort field with asc|desc"})

	rows := parser.Int("r", "rows", &argparse.Options{Required: false, Default: dftRows,
		Help: "Amount of docs that will be requested by one query and saved in one file"})

	dstDir := parser.String("d", "dst", &argparse.Options{Required: false, Default: ".",
		Help: "Path to place the dump directory"})

	user := parser.String("u", "user", &argparse.Options{Required: false, Default: "",
		Help: fmt.Sprintf("User name. That can be also set by %s environment variable", userEnv)})

	passw := parser.String("p", "password", &argparse.Options{Required: false, Default: "",
		Help: fmt.Sprintf("User password. That can be also set by %s environment variable", passwEnv)})

	httpTimeout := parser.Int("t", "httpTimeout", &argparse.Options{Required: false, Default: dftHTTPTimeout,
		Help: "http timeout in seconds"})

	strDirPerms := parser.String("m", "perms", &argparse.Options{Required: false, Default: dftDirPerms,
		Help: "Permissions for the dump directory"})

	if err = parser.Parse(os.Args); err != nil {
		return "", p, errors.New(parser.Usage(err))
	}

	// check collection link. SOLR-8642
	re := regexp.MustCompile(reLink)
	strs := re.FindStringSubmatch(*cLink)
	if strs == nil {
		msg := fmt.Sprintf("wrong http link to a solr collection \"%s\"\n", *cLink)
		return "", nil, errors.New(msg)
	}

	dirPerms, err := strconv.ParseUint(*strDirPerms, 8, 32)
	if err != nil {
		return "", nil, errors.New("wrong directory permissions")
	}

	// Get credentials from the environment if they aren't set in the command line
	if *user == "" {
		os.Getenv(userEnv)
	}

	if *passw == "" {
		os.Getenv(passwEnv)
	}

	p = &params{
		cLink:       *cLink,
		q:           *q,
		fl:          *fl,
		sort:        *sort,
		dstDir:      *dstDir,
		user:        *user,
		passw:       *passw,
		rows:        *rows,
		httpTimeout: *httpTimeout,
		dirPerms:    os.FileMode(dirPerms),
	}

	if strs[9] == "" {
		return strs[8], p, nil
	}
	return strs[9], p, nil
}

func rmVerField(docs []json.RawMessage) (data []byte, err error) {
	// adding the opening bracket

	data = append(data, []byte("[")...)

	// reading all docs and removing "_version_" filed
	for n, srcDoc := range docs {
		var tdoc map[string]interface{}
		if err := json.Unmarshal(srcDoc, &tdoc); err != nil {
			return nil, err
		}

		delete(tdoc, "_version_")

		dstDoc, err := json.Marshal(&tdoc)
		if err != nil {
			return nil, err
		}

		data = append(data, dstDoc...)
		// adding comma between docs
		if n < len(docs)-1 {
			data = append(data, []byte(",")...)
		}
	}
	// adding the closing bracket
	data = append(data, []byte("]\n")...)
	return data, nil
}

// Save the response with removing _version_ field
func saveSolrResp(dst io.Writer, emptyfl bool, v *glsolr.Response) (err error) {

	// data to save to file
	var data []byte
	// if "fl" parameter isn't set, removing _version_ field from the result
	if emptyfl {
		data, err = rmVerField(v.Response.Docs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error of Solr document proccessing. %s\n", err)
			return err
		}
	} else {
		data, err = json.Marshal(v.Response.Docs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error of Solr document proccessing. %s\n", err)
			return err
		}
		data = append(data, []byte("\n")...)
	}

	// write data to file
	if _, err := dst.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "error of writing to %s. %s", dst, err)
		return err
	}
	return nil
}

// define name pattern like PreffixServer.port.collection
func defNamePattern(colName, cLink string) (namePattern string, err error) {
	u, err := url.Parse(cLink)
	if err != nil {
		return "", err
	}

	host := strings.Replace(u.Host, ":", ".", 1)
	namePattern = fmt.Sprintf("%s.%s.", host, colName)
	return namePattern, nil
}

// Set parameters for a query
func qParams(p *params) (qp url.Values) {
	qp = url.Values{}
	qp.Set("q", p.q)
	qp.Set("sort", p.sort)
	if p.fl != "" {
		qp.Set("fl", p.fl)
	}
	qp.Set("rows", fmt.Sprintf("%d", p.rows))
	return qp
}

// Make directory for dumping
func mkDir(dstDir, namePattern string, perms os.FileMode) (fullDirPath string, err error) {
	// get directory name for dumping
	dirName := fmt.Sprintf("%s%s", namePattern, time.Now().Format(timeFmt))
	// get full path
	fullDirPath = path.Join(dstDir, dirName)
	// make directory
	if err := os.MkdirAll(fullDirPath, perms); err != nil {
		return "", err
	}
	return fullDirPath, nil
}

func getHeaders(agent, version string) (headers map[string]string) {
	headers = map[string]string{
		"User-Agent":      fmt.Sprintf("%s/%s (%s)", agent, version, runtime.GOOS),
		"Accept":          "application/json",
		"Connection":      "keep-alive",
		"Accept-Encoding": "gzip, deflate",
	}
	return headers
}

func main() {
	// parsing cli arguments
	colName, p, err := parceArgs(name, reLink, os.Args)
	if err != nil {
		// In case of error print error, print an error and exit
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(11)
	}

	// set parameters of query
	params := qParams(p)

	// define headers
	headers := getHeaders(name, version)

	// Create http client
	client := &http.Client{Timeout: time.Duration(p.httpTimeout) * time.Second}

	// Get the channel for receiving results
	results, err := glsolr.CursorSelect(p.cLink, p.user, p.passw, params, headers, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrong query. %s\n", err)
		os.Exit(-3)
	}

	// get name pattern like PreffixServer.port.collection
	namePattern, err := defNamePattern(colName, p.cLink)

	// make directory
	fullDirPath, err := mkDir(p.dstDir, namePattern, p.dirPerms)
	if err != nil {
		fmt.Fprintf(os.Stderr, "output error. %s\n", err)
		os.Exit(2)
	}

	// set initial status code
	statusCode := 0

	// initial number for file
	i := 1

	var wg sync.WaitGroup

	// Read results from the channel. Each reading starts the new cursor select query
	for res := range results {
		// Checking the type of result. If error is received, stopping.
		switch v := res.(type) {
		case error:
			fmt.Fprintf(os.Stderr, "query error. %s\n", v)
		case *glsolr.Response:
			// define full name of file to dump the result of one query
			fileName := fmt.Sprintf("%s%d.json", namePattern, i)
			fullFileName := filepath.Join(fullDirPath, fileName)
			i++
			// saving the results in parallel with launching the next query
			wg.Add(1)
			go func(fName string) {
				defer wg.Done()
				// create and open file
				dst, err := os.Create(fName)
				defer dst.Close()
				if err != nil {
					fmt.Fprintf(os.Stderr, "error of creating the file %s. %s\n", fName, err)
					return
				}
				if err := saveSolrResp(dst, p.fl == "", v); err != nil {
					statusCode = 10
				}
			}(fullFileName)
		}
	}
	// waiting all writing operations to files
	wg.Wait()
	// exit
	os.Exit(statusCode)
}
