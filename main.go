package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// 模板文件路径
	tpl := "./template/proto.go.tpl"

	// 要生成的数据库
	dbName := "test"

	// 生成文件路径
	file := "./template/" + dbName + ".proto"
	if IsFile(file) {
		fmt.Print("proto file already exist")
		return
	}

	// 数据库配置
	db, err := Connect("mysql", "root:toor@tcp(127.0.0.1:3306)/"+dbName+"?charset=utf8mb4&parseTime=true")
	if err != nil {
		fmt.Println(err)
	}

	// 不需要生成的表
	exclude := map[string]int{"log": 1}

	// 初始化表
	t := Table{}

	// 配置生成的 message
	t.Messages = map[string]Detail{
		"Filter": {
			Name:     "Filter",
			Category: "custom",
			Attrs: []Attr{{
				Type: "string", // 类型
				Name: "id",     // 字段
			}},
		},
		"Request": {
			Name:     "Request",
			Category: "all",
		},
		"Response": {
			Name:     "Response",
			Category: "custom",
			Attrs: []Attr{
				{
					Type: "string",
					Name: "id",
				},
				{
					Type: "bool",
					Name: "success",
				},
			},
		},
	}

	// 配置服务中的 RPC 方法
	t.Actions = map[string]Action{
		"Create": {Request: t.Messages["Request"], Response: t.Messages["Response"]},
		"Get":    {Request: t.Messages["Filter"], Response: t.Messages["Request"]},
		"Update": {Request: t.Messages["Request"], Response: t.Messages["Response"]},
		"Delete": {Request: t.Messages["Request"], Response: t.Messages["Response"]},
	}

	// 生成的包名
	t.PackageName = dbName

	// 生成的服务名
	t.ServiceName = StrFirstToUpper(dbName)

	// 处理数据库字段
	t.TableColumn(db, dbName, exclude)

	// 生成文件
	t.Generate(file, tpl)
}

// TableColumn 获取表信息
func (table *Table) TableColumn(db *sql.DB, dbName string, exclude map[string]int) {
	rows, err := db.Query("SELECT t.TABLE_NAME,t.TABLE_COMMENT,c.COLUMN_NAME,c.COLUMN_TYPE,c.COLUMN_COMMENT FROM information_schema.TABLES t,INFORMATION_SCHEMA.Columns c WHERE c.TABLE_NAME=t.TABLE_NAME AND t.`TABLE_SCHEMA`='" + dbName + "'")
	defer db.Close()
	defer rows.Close()
	var name, comment string
	var column Column
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	table.Comments = make(map[string]string)
	table.Names = make(map[string][]Column)
	for rows.Next() {
		rows.Scan(&name, &comment, &column.Name, &column.Type, &column.Comment)
		if _, ok := exclude[name]; ok {
			continue
		}
		if _, ok := table.Comments[name]; !ok {
			table.Comments[name] = comment
		}

		n := strings.Index(column.Type, "(")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		n = strings.Index(column.Type, " ")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		table.Names[name] = append(table.Names[name], column)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("error: %v", err)
		return
	}
}

// Generate 生成文件
func (table *Table) Generate(filepath, tpl string) {
	rpcservers := Service{Package: table.PackageName, Name: table.ServiceName}
	rpcservers.HandleFuncs(table)
	rpcservers.HandleMessage(table)
	tmpl, err := template.ParseFiles(tpl)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	err = tmpl.Execute(file, rpcservers)
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
}

func Connect(driverName, dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)

	if err != nil {
		log.Fatalln(err)
	}
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(30)
	if err := db.Ping(); err != nil {
		log.Fatalln(err)
	}
	return db, err
}

func (r *Service) HandleFuncs(t *Table) {
	var funcParam Function
	for key := range t.Comments {
		k := StrFirstToUpper(key)
		for n, m := range t.Actions {
			funcParam.Name = n + k
			funcParam.Path = strings.ToLower(k)
			funcParam.Method = FunctionMethod(strings.ToUpper(n))
			funcParam.ResponseName = k + m.Response.Name
			funcParam.RequestName = k + m.Request.Name
			r.Functions = append(r.Functions, funcParam)
		}
	}
}

func (r *Service) HandleMessage(t *Table) {
	message := Message{}
	field := Field{}
	var i int

	for key, value := range t.Names {
		k := StrFirstToUpper(key)

		for name, detail := range t.Messages {
			message.Name = k + name
			message.Detail = nil
			if detail.Category == "all" {
				i = 1
				for _, f := range value {
					field.Name = f.Name
					field.Type = TypeMToP(f.Type)
					if f.Type == "blob" {
						field.Type = "string"
						field.Comment = "; //用的时候要转成byte[] Convert.FromBase64String" + f.Comment
					} else {
						field.Comment = "; //" + f.Comment
					}
					field.Num = i
					message.Detail = append(message.Detail, field)
					i++
				}
			} else if detail.Category == "custom" {
				i = 1
				for _, f := range detail.Attrs {
					field.Type = f.Type
					field.Name = f.Name
					field.Num = i
					message.Detail = append(message.Detail, field)
					i++
				}
			}
			r.Messages = append(r.Messages, message)
		}
	}
}

func FunctionMethod(function string) string {
	getKeys := []string{"GET", "FIND", "QUERY", "LIST", "SEARCH"}
	postKeys := []string{"POST", "CREATE"}
	putKeys := []string{"PUT", "UPDATE"}
	deleteKeys := []string{"DELETE", "REMOVE"}
	if sliceContains(getKeys, function) {
		return "get"
	} else if sliceContains(postKeys, function) {
		return "post"
	} else if sliceContains(putKeys, function) {
		return "put"
	} else if sliceContains(deleteKeys, function) {
		return "delete"
	}
	return "post"
}

func sliceContains(s []string, str string) bool {
	for index := range s {
		if s[index] == str {
			return true
		}
	}
	return false
}

func TypeMToP(m string) string {
	if _, ok := typeArr[m]; ok {
		return typeArr[m]
	}
	return "string"
}

func StrFirstToUpper(str string) string {
	temp := strings.Split(str, "_")
	var upperStr string
	for _, v := range temp {
		if len(v) > 0 {
			runes := []rune(v)
			upperStr += string(runes[0]-32) + string(runes[1:])
		}
	}
	return upperStr
}

func IsFile(f string) bool {
	fi, e := os.Stat(f)
	if e != nil {
		return false
	}
	return !fi.IsDir()
}

var typeArr = map[string]string{
	"int":       "int32",
	"tinyint":   "int32",
	"smallint":  "int32",
	"mediumint": "int32",
	"enum":      "int32",
	"bigint":    "int64",
	"varchar":   "string",
	"timestamp": "google.protobuf.Timestamp",
	"date":      "string",
	"text":      "string",
	"double":    "double",
	"decimal":   "double",
	"float":     "float",
	"datetime":  "google.protobuf.Timestamp",
	"blob":      "blob",
}

type Table struct {
	PackageName string
	ServiceName string
	Actions     map[string]Action
	Comments    map[string]string
	Names       map[string][]Column
	Messages    map[string]Detail
}

type Action struct {
	Request  Detail
	Response Detail
}
type Column struct {
	Name    string
	Type    string
	Comment string
}

type Service struct {
	Package   string
	Name      string
	Functions []Function
	Messages  []Message
}

type Function struct {
	Name         string
	Path         string
	Method       string
	RequestName  string
	ResponseName string
}

type Message struct {
	Name   string
	Detail []Field
}

type Field struct {
	Type    string
	Name    string
	Num     int
	Comment string
}

type Detail struct {
	Name     string
	Category string // all or custom
	Attrs    []Attr
}

type Attr struct {
	Type string
	Name string
}
