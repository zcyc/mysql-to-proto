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
	dbName := "social"

	// 生成文件路径
	file := "./template/" + dbName + ".proto"
	if IsFile(file) {
		fmt.Print("proto file already exist")
		return
	}

	// 数据库配置
	db, err := Connect("mysql", "root:root@tcp(10.0.0.103:3306)/"+dbName+"?charset=utf8mb4&parseTime=true")
	if err != nil {
		fmt.Println(err)
	}

	// 不需要生成的表
	exclude := map[string]int{"log": 1}

	// 初始化表
	t := Table{}

	// 配置生成的 message
	t.Message = map[string]Detail{
		"Filter": {
			Name: "Filter",
			Cat:  "custom",
			Attr: []AttrDetail{{
				TypeName: "string", // 类型
				AttrName: "id",     // 字段
			}},
		},
		"Request": {
			Name: "Request",
			Cat:  "all",
		},
		"Response": {
			Name: "Response",
			Cat:  "custom",
			Attr: []AttrDetail{
				{
					TypeName: "string",
					AttrName: "id",
				},
				{
					TypeName: "bool",
					AttrName: "success",
				},
			},
		},
	}

	// 配置服务中的 RPC 方法
	t.Method = map[string]MethodDetail{
		"Get":    {Request: t.Message["Filter"], Response: t.Message["Request"]},
		"Create": {Request: t.Message["Request"], Response: t.Message["Response"]},
		"Update": {Request: t.Message["Request"], Response: t.Message["Response"]},
	}

	// 生成的包名
	t.PackageModels = dbName

	// 生成的服务名
	t.ServiceName = dbName + "Service"

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
	table.Comment = make(map[string]string)
	table.Name = make(map[string][]Column)
	for rows.Next() {
		rows.Scan(&name, &comment, &column.Field, &column.Type, &column.Comment)
		if _, ok := exclude[name]; ok {
			continue
		}
		if _, ok := table.Comment[name]; !ok {
			table.Comment[name] = comment
		}

		n := strings.Index(column.Type, "(")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		n = strings.Index(column.Type, " ")
		if n > 0 {
			column.Type = column.Type[0:n]
		}
		table.Name[name] = append(table.Name[name], column)
	}

	if err = rows.Err(); err != nil {
		fmt.Printf("error: %v", err)
		return
	}
}

// Generate 生成文件
func (table *Table) Generate(filepath, tpl string) {
	rpcservers := RpcServers{Models: table.PackageModels, Name: table.ServiceName}
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

func (r *RpcServers) HandleFuncs(t *Table) {
	var funcParam FuncParam
	for key := range t.Comment {
		k := StrFirstToUpper(key)
		for n, m := range t.Method {
			funcParam.Name = n + k
			funcParam.ResponseName = k + m.Response.Name
			funcParam.RequestName = k + m.Request.Name
			r.Funcs = append(r.Funcs, funcParam)
		}
	}
}

func (r *RpcServers) HandleMessage(t *Table) {
	message := Message{}
	field := Field{}
	var i int

	for key, value := range t.Name {
		k := StrFirstToUpper(key)

		for name, detail := range t.Message {
			message.Name = k + name
			message.MessageDetail = nil
			if detail.Cat == "all" {
				i = 1
				for _, f := range value {
					field.AttrName = f.Field
					field.TypeName = TypeMToP(f.Type)
					if f.Type == "blob" {
						field.TypeName = "string"
						field.Comment = "; //用的时候要转成byte[] Convert.FromBase64String" + f.Comment
					} else {
						field.Comment = "; //" + f.Comment
					}
					field.Num = i
					message.MessageDetail = append(message.MessageDetail, field)
					i++
				}
			} else if detail.Cat == "custom" {
				i = 1
				for _, f := range detail.Attr {
					field.TypeName = f.TypeName
					field.AttrName = f.AttrName
					field.Num = i
					message.MessageDetail = append(message.MessageDetail, field)
					i++
				}
			}
			r.MessageList = append(r.MessageList, message)
		}
	}
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
	PackageModels string
	ServiceName   string
	Method        map[string]MethodDetail
	Comment       map[string]string
	Name          map[string][]Column
	Message       map[string]Detail
}

type MethodDetail struct {
	Request  Detail
	Response Detail
}
type Column struct {
	Field   string
	Type    string
	Comment string
}

type RpcServers struct {
	Models      string
	Name        string
	Funcs       []FuncParam
	MessageList []Message
}

type FuncParam struct {
	Name         string
	RequestName  string
	ResponseName string
}

type Message struct {
	Name          string
	MessageDetail []Field
}

type Field struct {
	TypeName string
	AttrName string
	Num      int
	Comment  string
}

type Detail struct {
	Name string
	Cat  string // all or custom
	Attr []AttrDetail
}

type AttrDetail struct {
	TypeName string
	AttrName string
}
