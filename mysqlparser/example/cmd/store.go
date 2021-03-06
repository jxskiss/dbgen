package main

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"text/template"

	parser "github.com/jxskiss/dbgen/mysqlparser"
)

func generateDAOs(args *Args, tables []*parser.Table) {
	var code []byte
	for _, t := range tables {
		code = generateDAOCode(args, t)
		if len(code) == 0 {
			continue
		}

		daoPkgName := getBasePkgName(args.DAOPkg)
		customDAOFile := getFileName(args.DAOPkg, t.Name+"_store.go")
		genDAOFile := getFileName(args.DAOPkg, t.Name+"_store_gen.go")

		log.Printf("writing dao file: %s", genDAOFile)
		err := os.WriteFile(genDAOFile, code, 0644)
		assertNil(err)

		touchCustomDAOFile(customDAOFile, daoPkgName, t)
	}
}

var customDAOTmpl = `
package %s

type %sCustomMethods interface {
}
`

func touchCustomDAOFile(filename string, pkgName string, table *parser.Table) {
	if _, err := os.Stat(filename); err == nil || !os.IsNotExist(err) {
		return
	}

	code := []byte(fmt.Sprintf(customDAOTmpl, pkgName, table.VarName()))
	code, _ = format.Source(code)

	log.Printf("writing dao file: %s", filename)
	err := os.WriteFile(filename, code, 0644)
	assertNil(err)
}

func generateDAOCode(args *Args, t *parser.Table) []byte {
	var err error
	var buf bytes.Buffer

	pkgName := getBasePkgName(args.DAOPkg)
	headerData := map[string]interface{}{
		"PkgName": pkgName,
	}
	if args.DAOPkg != args.ModelPkg {
		headerData["ModelPkg"] = args.ModelPkg
	}
	err = headerTmpl.Execute(&buf, headerData)
	assertNil(err)

	daoMethods := getDAOMethods(args, t)
	err = storeTmpl.ExecuteTemplate(&buf, "dao", map[string]interface{}{
		"Table":   t,
		"Methods": daoMethods,
	})
	assertNil(err)

	queries := t.Queries()
	for _, q := range queries {
		var tmpl string
		var data interface{}
		switch q {
		case "Get", "GetWhere", "MGet", "MGetWhere", "Create", "Update":
			tmpl = q
			data = t
		default:
			cq := parser.ParseQuery(t, q)
			if cq.IsMGet() {
				tmpl = "customMGet"
			} else {
				tmpl = "customGet"
			}
			data = map[string]interface{}{
				"Table": t,
			}
		}
		err = storeTmpl.ExecuteTemplate(&buf, tmpl, data)
		assertNil(err)
	}

	code := buf.Bytes()
	if !args.DisableFormat {
		code, err = format.Source(code)
		assertNil(err)
	}
	return code
}

func getDAOMethods(args *Args, t *parser.Table) (methods []string) {
	pkgPrefix := ""
	if args.ModelPkg != args.DAOPkg {
		modelPkgName := getBasePkgName(args.ModelPkg)
		pkgPrefix = modelPkgName + "."
	}

	queries := t.Queries()
	for _, q := range queries {
		switch q {
		case "Get":
			sig := fmt.Sprintf("Get(ctx context.Context, %s int64, opts ...dbgen.Opt) (*%s%s, error)",
				t.PKVarName(), pkgPrefix, t.TypeName())
			methods = append(methods, sig)
		case "GetWhere":
			sig := fmt.Sprintf("GetWhere(ctx context.Context, where string, paramsAndOpts ...interface{}) (*%s%s, error)",
				pkgPrefix, t.TypeName())
			methods = append(methods, sig)
		case "MGet":
			sig := fmt.Sprintf("MGet(ctx context.Context, %sList []int64, opts ...dbgen.Opt) (%s%sList, error)",
				t.PKVarName(), pkgPrefix, t.TypeName())
			methods = append(methods, sig)
		case "MGetWhere":
			sig := fmt.Sprintf("MGetWhere(ctx context.Context, where string, paramsAndOpts ...interface{}) (%s%sList, error)",
				pkgPrefix, t.TypeName())
			methods = append(methods, sig)
		case "Create":
			sig := fmt.Sprintf("Create(ctx context.Context, %s *%s%s, opts ...dbgen.Opt) error",
				t.VarName(), pkgPrefix, t.TypeName())
			methods = append(methods, sig)
		case "Update":
			sig := fmt.Sprintf("Update(ctx context.Context, %s int64, updates map[string]interface{}, opts ...dbgen.Opt) error",
				t.PKVarName())
			methods = append(methods, sig)
		default:
			cq := parser.ParseQuery(t, q)
			if cq.IsMGet() {
				sig := fmt.Sprintf("%s(ctx context.Context, %s, opts ...dbgen.Opt) (%s%sList, error)",
					cq.Name, cq.ArgList(), pkgPrefix, t.TypeName())
				methods = append(methods, sig)
			} else {
				sig := fmt.Sprintf("%s(ctx context.Context, %s, opts ...dbgen.Opt) (*%s%s, error)",
					cq.Name, cq.ArgList(), pkgPrefix, t.TypeName())
				methods = append(methods, sig)
			}
		}
	}
	return
}

// -------- templates -------- //

var storeTmpl = &template.Template{}

func init() {
	mustParse := func(name, text string) {
		template.Must(storeTmpl.New(name).Parse(text))
	}

	mustParse("dao", `
const {{ .Table.TableNameConst }} = "{{ .Table.Name }}"

type {{ .Table.TypeName }}DAO interface {
	{{- range .Methods }}
	{{ . }}
	{{- end }}
	{{ .Table.VarName }}CustomMethods
}

func Get{{ .Table.TypeName }}DAO(conn dbgen.MySQLConn) {{ .Table.TypeName }}DAO {
	return &{{ .Table.DaoImplName }}{
		db: conn,
	}
}

type {{ .Table.DaoImplName }} struct {
	db *gorm.DB
}
`)

	mustParse("Get", `
func (p *{{ .DaoImplName }}) Get(ctx context.Context, {{ .PKVarName }} int64, opts ...dbgen.Opt) (*{{ .PkgPrefix }}{{ .TypeName }}, error) {
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out = &{{ .PkgPrefix }}{{ .TypeName }}{}
	err := conn.WithContext(ctx).Table(tableName).Where("{{ .PrimaryKey }} = ?", {{ .PKVarName }}).First(out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)

	mustParse("GetWhere", `
func (p *{{ .DaoImplName }}) GetWhere(ctx context.Context, where string, paramsAndOpts ...interface{}) (*{{ .PkgPrefix }}{{ .TypeName }}, error) {
	params, opts := dbgen.SplitOpts(paramsAndOpts)
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out = &{{ .PkgPrefix }}{{ .TypeName }}{}
	err := conn.WithContext(ctx).Table(tableName).Where(where, params...).First(out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)

	mustParse("MGet", `
func (p *{{ .DaoImplName }}) MGet(ctx context.Context, {{ .PKVarName }}List []int64, opts ...dbgen.Opt) ({{ .PkgPrefix }}{{ .TypeName }}List, error) {
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out {{ .PkgPrefix }}{{ .TypeName }}List
	err := conn.WithContext(ctx).Table(tableName).Where("{{ .PrimaryKey }} in (?)", {{ .PKVarName }}List).Find(&out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)
	mustParse("MGetWhere", `
func (p *{{ .DaoImplName }}) MGetWhere(ctx context.Context, where string, paramsAndOpts ...interface{}) ({{ .PkgPrefix }}{{ .TypeName }}List, error) {
	params, opts := dbgen.SplitOpts(paramsAndOpts)
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out {{ .PkgPrefix }}{{ .TypeName }}List
	err := conn.WithContext(ctx).Table(tableName).Where(where, params...).Find(&out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)

	mustParse("Create", `
func (p *{{ .DaoImplName }}) Create(ctx context.Context, {{ .VarName }} *{{ .PkgPrefix }}{{ .TypeName }}, opts ...dbgen.Opt) error {
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	err := conn.WithContext(ctx).Table(tableName).Create({{ .VarName }}).Error
	if err != nil {
		return errors.AddStack(err)
	}
	return nil
}
`)

	mustParse("Update", `
func (p *{{ .DaoImplName }}) Update(ctx context.Context, {{ .PKVarName }} int64, updates map[string]interface{}, opts ...dbgen.Opt) error {
	if len(updates) == 0 {
		return errors.New("programming error: empty updates map")
	}
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	err := conn.WithContext(ctx).Table(tableName).Where("{{ .PrimaryKey }} = ?", {{ .PKVarName }}).Updates(updates).Error
	if err != nil {
		return errors.AddStack(err)
	}
	return nil
}
`)

	mustParse("customGet", `
func (p *{{ .Table.DaoImplName }}) {{ .FuncName }}(ctx context.Context, {{ .ArgList }}, opts ...dbgen.Opt) (*{{ .Table.PkgPrefix }}{{ .Table.TypeName }}, error) {
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out = &{{ .Table.PkgPrefix }}{{ .Table.TypeName }}{}
	err := conn.WithContext(ctx).Table(tableName).
		Where({{ .Where }}).
		First(out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)

	mustParse("customMGet", `
func (p *{{ .Table.DaoImplName }}) {{ .FuncName }}(ctx context.Context, {{ .ArgList }}, opts ...dbgen.Opt) ({{ .Table.PkgPrefix }}{{ .Table.TypeName }}List, error) {
	conn := dbgen.GetSession(p.db, opts...)
	tableName := {{ .TableNameConst }}
	var out {{ .Table.PkgPrefix }}{{ .Table.TypeName }}List
	err := conn.WithContext(ctx).Table(tableName).
		Where({{ .Where }}).
		Find(&out).Error
	if err != nil {
		return nil, errors.AddStack(err)
	}
	return out, nil
}
`)
}
