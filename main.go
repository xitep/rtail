package main

import (
	"github.com/xitep/gopass"
	"crypto/tls"
	"errors"
	"fmt"
	flag "github.com/ogier/pflag"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	version = "1.2-dev"
)

func main() {
	// ~ parse command line
	user := flag.StringP("user", "u", "", "Specify the username")
	password := flag.StringP("password", "p", "", "Specify the password")
	askPassword := flag.Bool("ask-password", false, "Specify password interactively")
	dumpHeaders := flag.Bool("dump-headers", false, "Dump request/response headers to stderr")
	follow := flag.BoolP("follow", "f", false, "Output appended data as the remote file grows")
	sleep := flag.IntP("sleep-interval", "s", 5, "With --follow check for appendeded data approximately every N seconds")
	printVersion := flag.BoolP("version", "", false, "Print version and quit")
	lbytes := flag.StringP("bytes", "c", "1K", "output the last N bytes")
	output := flag.StringP("output", "o", "-", "Write output to named file")
	separator := flag.StringP("separator", "", "", "Write separator between outputs")

	flag.Usage = usage
	flag.Parse()

	if *printVersion {
		fmt.Printf("%s\n", version)
		os.Exit(0)
	}

	if *sleep <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid --sleep-interval=%d: Must be greater than zero!", *sleep)
		os.Exit(1)
	}
	if !*follow {
		*sleep = -1 // ~ causes client not to repeatadly scan the remote file
	}

	lbytesValue, lbytesValueFromStart, err := parseByteSize(*lbytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if *output == "" {
		fmt.Fprintf(os.Stderr, "Invalid --output option; Must have an argument!")
		os.Exit(1)
	}
	if *output == "-" {
		// ~ stdout intended
		*output = ""
	}

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	resource := flag.Arg(0)

	if *askPassword {
		if len(*user) == 0 {
			fmt.Fprintf(os.Stderr, "No user specified! Use -user option!")
			os.Exit(1)
		}
		var err error
		*password, err = credentials(resource, *user)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}

	// ~ prepare 'tail' client
	client := client{
		user:     *user,
		password: *password,

		c: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				TLSHandshakeTimeout: 10 * time.Second,
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).Dial,
			},
		},
		userAgent:           "rtail/" + version,
		outputFile:          *output,
		separator:           *separator,
		tick:                *sleep,
		byteOffset:          lbytesValue,
		byteOffsetFromStart: lbytesValueFromStart,
		dumpHeaders:         *dumpHeaders,
	}

	// ~ run the 'tail' client
	if err := client.tail(resource); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] URL\n", os.Args[0])
	flag.PrintDefaults()
}

// Asks the user for credientials to authenticate the fetching the
// remote resource/subject.
func credentials(subject string, user string) (string, error) {
	pwd, err := gopass.GetPass(fmt.Sprintf("password for %s: ", user))
	if err != nil {
		return "", err
	}
	return pwd, nil
}

// --------------------------------------------------------------------

var sizeSuffixes = []struct {
	s string
	m int64
}{
	{"", 1}, // the "no-suffix" case

	{"b", 512},

	{"kB", 1000},
	{"KB", 1000},

	{"K", 1024},
	{"KiB", 1024},

	{"mB", 1000 * 1000},
	{"MB", 1000 * 1000},

	{"M", 1024 * 1024},
	{"MiB", 1024 * 1024},

	{"gB", 1000 * 1000 * 1000},
	{"GB", 1000 * 1000 * 1000},

	{"G", 1024 * 1024 * 1024},
	{"GiB", 1024 * 1024 * 1024},
}

// Returns 0 if s cannot be found, otherwise
// the found suffix's multiplier
func findSizeSuffix(suffix string) int64 {
	for _, ss := range sizeSuffixes {
		if ss.s == suffix {
			return ss.m
		}
	}
	return 0
}

// Parses the given string as the value supplied to the --bytes
// option.  A successful parse will deliver a value >= 0 and
// true iff the string begins with a '+'.
func parseByteSize(s string) (int64, bool, error) {
	sIndex := len(s)

	plus := false
	if sIndex > 0 && s[0] == '+' {
		s = s[1:]
		sIndex--
		plus = true
	}

	var size int64
	for i, c := range s {
		if '0' <= c && c <= '9' {
			size = size*10 + int64(c-'0')
		} else {
			sIndex = i
			break
		}
	}
	m := findSizeSuffix(s[sIndex:])
	if sIndex == 0 || m == 0 {
		// sIndex == 0: no digits at the beginning or len(s) == 0
		// m == 0: unknown suffix
		return 0, false,
			errors.New(fmt.Sprintf("%q is not a valid number of bytes", s))
	}
	return size * m, plus, nil
}
