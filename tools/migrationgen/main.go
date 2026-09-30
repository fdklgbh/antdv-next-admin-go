package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var validName = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func main() {
	name := flag.String("name", "", "snake_case migration name")
	flag.Parse()
	if !validName.MatchString(*name) {
		fmt.Fprintln(os.Stderr, "name must be non-empty snake_case")
		os.Exit(2)
	}
	version := time.Now().UTC().Format("20060102150405")
	functionName := "migration" + version + pascal(*name)
	content := fmt.Sprintf("package migrations\n\nimport (\n\t\"github.com/go-gormigrate/gormigrate/v2\"\n\t\"gorm.io/gorm\"\n)\n\nfunc %s() *gormigrate.Migration {\n\treturn &gormigrate.Migration{\n\t\tID: \"%s_%s\",\n\t\tMigrate: func(tx *gorm.DB) error {\n\t\t\treturn nil\n\t\t},\n\t\tRollback: func(tx *gorm.DB) error {\n\t\t\treturn nil\n\t\t},\n\t}\n}\n", functionName, version, *name)
	path := filepath.Join("migrations", "v"+version+"_"+*name+".go")
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(path, functionName)
}
func pascal(value string) string {
	parts := strings.Split(value, "_")
	for index := range parts {
		parts[index] = strings.ToUpper(parts[index][:1]) + parts[index][1:]
	}
	return strings.Join(parts, "")
}
