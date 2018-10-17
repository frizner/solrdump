# solrdump
[![Go Report Card](https://goreportcard.com/badge/github.com/frizner/solrdump)](https://goreportcard.com/report/github.com/frizner/solrdump)

**solrdump** fetches documents from a Solr collection using a cursor query and saves them as JSON files in a new directory:
```sh
$ solrdump -c "http://solrsrv01:8983/solr/gettingstarted" -r 10 -s "id asc"
$ ls
$ solrsrv01.8983.gettingstarted.20181017-160227
$ ls solrsrv01.8983.gettingstarted.20181017-160227
$ $ ls -1 solrsrv01.8983.gettingstarted.20181017-160227/
solrsrv01.8983.gettingstarted.1.json
solrsrv01.8983.gettingstarted.2.json
solrsrv01.8983.gettingstarted.3.json
solrsrv01.8983.gettingstarted.4.json
solrsrv01.8983.gettingstarted.5.json
solrsrv01.8983.gettingstarted.6.json
```

## Feauteres
- Requesting the documents from a Solr collection using [a cursor. query](https://lucene.apache.org/solr/guide/pagination-of-results.html) in order to avoid the problem of ["Deep paging"](https://lucene.apache.org/solr/guide/pagination-of-results.html#performance-problems-with-deep-paging).
- Requesting and saving the results are being doing in parallel.
- `Field list` parameter can be empty. In this case **solrdump** will export documents with all fields removing only `_version_` field.
## Constraints
  - `start` parameter can not be used because is mutually exclusive with the `cursor`.
  - `sort` parameter is mandatory and must include the `uniqueKey` field (either `asc` or `desc`).
[More info.](https://lucene.apache.org/solr/guide/7_5/pagination-of-results.html#constraints-when-using-cursors)

## Installation
### Binaries
Download the binary from the [releases](https://github.com/frizner/solrdump/releases) page.
### From Source
You can use the go tool to install `solrdump`:
```sh
$ go get "github.com/frizner/solrdump"
$ go install "github.com/frizner/cmd/solrdump"
```
This installs the command into the bin sub-folder of wherever your $GOPATH environment variable points. If this directory is already in your $PATH, then you should be good to go.

If you have already pulled down this repo to a location that is not in your $GOPATH and want to build from the sources, you can cd into the repo and then run make install.

## Usage
```sh
$ ~/go/bin/solrdump -h
usage: gosolrdump [-h|--help] -c|--colllink "<value>" [-q|--query "<value>"]
                  [-f|--fieldlist "<value>"] -s|--sort "<value>" [-r|--rows
                  <integer>] [-d|--dst "<value>"] [-u|--user "<value>"]
                  [-p|--password "<value>"] [-t|--httpTimeout <integer>]
                  [-m|--perms "<value>"]

                  gosolrdump fetches documents from a Solr collection (index)
                  using a cursor query and exports them to json files 

Arguments:

  -h  --help         Print help information
  -c  --colllink     http link to a Solr collection like
                     http[s]://address[:port]/solr/collection
  -q  --query        Q parameter. Default: *:*
  -f  --fieldlist    Fields list. All fields of documents are exported by
                     default. Default: 
  -s  --sort         Sort field with asc|desc
  -r  --rows         Amount of docs that will be requested by one query and
                     saved in one file. Default: 100000
  -d  --dst          Path to place the dump directory. Default: .
  -u  --user         User name. That can be also set by SOLRUSER environment
                     variable. Default: 
  -p  --password     User password. That can be also set by SOLRPASSW
                     environment variable. Default: 
  -t  --httpTimeout  http timeout in seconds. Default: 180
  -m  --perms        Permissions for the dump directory. Default: 0755
```

### License
`solrdump` is released under the MIT License. See [LICENSE](https://github.com/frizner/solrdump/blob/master/LICENSE).
