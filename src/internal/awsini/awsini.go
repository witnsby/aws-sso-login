// Package awsini holds the go-ini settings used to read and write the AWS
// config and shared credentials files, so every reader and writer in this
// module treats them the way AWS CLI v2 (botocore's configparser) does.
package awsini

import "github.com/go-ini/ini"

// init sets go-ini's process-wide write style. This is the only place in the
// module that sets it: keys are written as "key = value" without padding
// them into aligned columns, so rewriting the credentials file does not
// re-format sections it did not change.
func init() {
	ini.PrettyFormat = false
	ini.PrettyEqual = true
}

// LoadOptions returns the options for loading AWS config and credentials
// files. botocore reads them with configparser.RawConfigParser, which keeps
// inline '#' and ';' and surrounding double quotes as part of a value; only
// lines starting with '#' or ';' are comments.
func LoadOptions() ini.LoadOptions {
	return ini.LoadOptions{
		IgnoreInlineComment:     true,
		PreserveSurroundedQuote: true,
	}
}

// Load reads the INI file at path with LoadOptions. A missing file yields
// an error that matches errors.Is(err, fs.ErrNotExist).
func Load(path string) (*ini.File, error) {
	return ini.LoadSources(LoadOptions(), path)
}
