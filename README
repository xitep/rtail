## Remote tail

A 'tail' for (remote) files served over http/https.  The program
repeatedly asks a web server for the content of a given URL using
"range" requests and prints the content out to standard output.


### Installation

To build the tool from sources you'll need Go 1.3.  Using the 'go'
tool the following command in the project directory builds the binary:

   $ go build


### Usage

   $ rtail -f http://myserver/myapplication/mylog.log

For more see output of `rtail` with the `--help` option.  Command line
options are intentionally similar to those of the coreutils' classic
`tail` program.
