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
	d := Database{}

	// 配置生成的 message
	d.Details = map[string]Detail{
		"Filter": {
			Name:     "Filter",
			Category: "custom",
			Attributes: []Attribute{
				{
					Type: "string", // 类型
					Name: "id",     // 字段
				},
			},
		},
		"Page": {
			Name:     "Page",
			Category: "custom",
			Attributes: []Attribute{
				{
					Type:    "int32", // 类型
					Name:    "page",  // 字段
					Comment: "第几页",
				},
				{
					Type:    "int32",
					Name:    "page_size",
					Comment: "每页数量",
				},
			},
		},
		"Request": {
			Name:     "Request",
			Category: "all",
		},
		"Response": {
			Name:     "Response",
			Category: "custom",
			Attributes: []Attribute{
				{
					Type:    "int32",
					Name:    "code",
					Comment: "错误码",
				},
				{
					Type:    "string",
					Name:    "message",
					Comment: "错误信息",
				},
				{
					Type:    "string",
					Name:    "data",
					Comment: "返回值，使用前将类型改成 any",
				},
			},
		},
	}

	// 配置服务中的 RPC 方法
	d.Actions = map[string]Action{
		"Create": {Request: d.Details["Request"], Response: d.Details["Response"]},
		"List":   {Request: d.Details["Page"], Response: d.Details["Response"]},
		"Get":    {Request: d.Details["Filter"], Response: d.Details["Response"]},
		"Update": {Request: d.Details["Request"], Response: d.Details["Response"]},
		"Delete": {Request: d.Details["Filter"], Response: d.Details["Response"]},
	}

	// 生成的包名
	d.Name = dbName

	// 处理数据库字段
	d.TableColumn(db, dbName, exclude)

	// 生成文件
	d.Generate(file, tpl)
}

// TableColumn 获取表信息
func (d *Database) TableColumn(db *sql.DB, dbName string, exclude map[string]int) {
	rows, err := db.Query("SELECT t.TABLE_NAME,t.TABLE_COMMENT,c.COLUMN_NAME,c.COLUMN_TYPE,c.COLUMN_COMMENT FROM information_schema.TABLES t,INFORMATION_SCHEMA.Columns c WHERE c.TABLE_NAME=t.TABLE_NAME AND t.`TABLE_SCHEMA`='" + dbName + "'")
	defer db.Close()
	defer rows.Close()
	var name, comment string
	var column Column
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	d.Comments = make(map[string]string)
	d.Tables = make(map[string][]Column)
	for rows.Next() {
		rows.Scan(&name, &comment, &column.Name, &column.Type, &column.Comment)
		if _, ok := exclude[name]; ok {
			continue
		}
		if _, ok := d.Comments[name]; !ok {
			d.Comments[name] = comment
		}

		n := strings.Index(column.Type, "(")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		n = strings.Index(column.Type, " ")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		d.Tables[name] = append(d.Tables[name], column)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("error: %v", err)
		return
	}
}

// Generate 生成文件
func (d *Database) Generate(filepath, tpl string) {
	protoBuff := ProtoBuff{Package: d.Name}
	for tableName := range d.Comments {
		service := Service{Name: StrFirstToUpper(tableName)}
		service.HandleFuncs(d.Actions, tableName)
		service.HandleMessage(d.Details, tableName, d.Tables[tableName])
		protoBuff.Services = append(protoBuff.Services, service)
	}

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
	err = tmpl.Execute(file, protoBuff)
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

func (s *Service) HandleFuncs(actions map[string]Action, key string) {
	k := StrFirstToUpper(key)
	for n, m := range actions {
		var funcParam Function
		funcParam.Name = n + k
		funcParam.Path = strings.ToLower(k)
		funcParam.Method = FunctionMethod(strings.ToUpper(n))
		funcParam.ResponseName = k + m.Response.Name
		funcParam.RequestName = k + m.Request.Name

		// 特殊处理 url
		if n == "List" {
			funcParam.Path = strings.ToLower(k) + "/list"
		}

		s.Functions = append(s.Functions, funcParam)
	}
}

func (s *Service) HandleMessage(details map[string]Detail, key string, columns []Column) {
	var message Message
	k := StrFirstToUpper(key)

	for name, detail := range details {
		message.Name = k + name

		// 这里必须清空一下
		message.Detail = nil

		// 处理数据表全部列消息体
		if detail.Category == "all" {
			for i, f := range columns {
				var field Field
				field.Name = f.Name
				field.Type = TypeMToP(f.Type)
				field.Num = i + 1
				if f.Type == "blob" {
					if f.Comment != "" {
						field.Type = "string"
						field.Comment = "; // 用的时候要转成byte[] Convert.FromBase64String" + f.Comment
					} else {
						field.Type = "string"
						field.Comment = "; // 用的时候要转成byte[] Convert.FromBase64String"
					}
				} else {
					if f.Comment != "" {
						field.Comment = "; // " + f.Comment
					} else {
						field.Comment = ";"
					}
				}
				message.Detail = append(message.Detail, field)
			}
		} else if detail.Category == "custom" {
			// 处理自定义消息体
			for i, f := range detail.Attributes {
				var field Field
				field.Type = f.Type
				field.Name = f.Name
				field.Num = i + 1
				field.Type = TypeMToP(f.Type)
				if f.Type == "blob" {
					if f.Comment != "" {
						field.Type = "string"
						field.Comment = "; // 用的时候要转成byte[] Convert.FromBase64String，" + f.Comment
					} else {
						field.Type = "string"
						field.Comment = "; // 用的时候要转成byte[] Convert.FromBase64String"
					}
				} else {
					if f.Comment != "" {
						field.Comment = "; // " + f.Comment
					} else {
						field.Comment = ";"
					}
				}
				message.Detail = append(message.Detail, field)
			}
		}
		s.Messages = append(s.Messages, message)
	}
}

// 获取方法的请求方式
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

// 判断切片是否包含
func sliceContains(s []string, str string) bool {
	for index := range s {
		if s[index] == str {
			return true
		}
	}
	return false
}

// MySQL 类型转 PB 类型
func TypeMToP(m string) string {
	if _, ok := typeArr[m]; ok {
		return typeArr[m]
	}
	return "string"
}

// MySQL 类型和 PB 类型映射表
var typeArr = map[string]string{
	"int":       "int32",
	"tinyint":   "int32",
	"smallint":  "int32",
	"mediumint": "int32",
	"enum":      "int32",
	"bigint":    "int64",
	"varchar":   "string",
	"timestamp": "string",
	"date":      "string",
	"text":      "string",
	"double":    "double",
	"decimal":   "double",
	"float":     "float",
	"datetime":  "string",
	"blob":      "blob",
}

// 单词首字母转大写
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

type Database struct {
	Name     string
	Actions  map[string]Action
	Comments map[string]string
	Tables   map[string][]Column
	Details  map[string]Detail
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

type ProtoBuff struct {
	Package  string
	Services []Service
}

type Service struct {
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
	Name       string
	Category   string
	Attributes []Attribute
}

type Attribute struct {
	Type    string
	Name    string
	Comment string
}
