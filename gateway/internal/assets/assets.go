package assets

import _ "embed"

//go:embed migrations/001_schema.sql
var Schema string

//go:embed data/cases.json
var CasesJSON []byte
