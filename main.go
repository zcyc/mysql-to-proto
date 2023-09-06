package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"
	"unicode"

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

	// 需要生成的表
	include := map[string]struct{}{
		// "user1": {},
	}

	// 不需要生成的表，需要生成的表为空时生效
	exclude := map[string]struct{}{
		"test": {},
	}

	// 初始化表
	d := Database{}

	// 生成的包名
	d.Name = dbName

	// 处理数据库字段
	d.TableColumn(db, dbName, include, exclude)

	// 生成文件
	d.Generate(file, tpl)
}

// 获取表信息
func (d *Database) TableColumn(db *sql.DB, dbName string, include, exclude map[string]struct{}) {
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
		// 如果有 include 则生成 include 包含的表，如果没有 include 则生成 exclude 不包含的表
		if len(include) != 0 {
			if _, ok := include[name]; !ok {
				continue
			}
		} else {
			if _, ok := exclude[name]; ok {
				continue
			}
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

// 生成文件
func (d *Database) Generate(filepath, tpl string) {
	protoBuff := ProtoBuff{Package: d.Name}
	for tableName := range d.Comments {
		camelTableName := UpperCamel(tableName)
		service := Service{Name: camelTableName}
		service.HandleFunction(camelTableName)
		service.HandleMessage(camelTableName, d.Tables[tableName])
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

// 连接数据库
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

// 处理函数
func (s *Service) HandleFunction(tableName string) {
	var function Function
	function.Name = tableName
	function.Path = Kebab(tableName)
	function.Method = "patch"
	function.ResponseName = tableName + "Request"
	function.RequestName = tableName + "Request"
	s.Function = function
}

// 处理 message
func (s *Service) HandleMessage(tableName string, columns []Column) {
	var message Message
	for i, f := range columns {
		var field Field
		field.Name = f.Name
		field.Type = TypeSqlToPb(f.Type)
		field.Num = i + 1
		if f.Type == "blob" {
			if f.Comment != "" {
				field.Type = "string"
				field.Comment = "; // 使用时要转成 []byte" + f.Comment
			} else {
				field.Type = "string"
				field.Comment = "; // 使用时要转成 []byte"
			}
		} else {
			if f.Comment != "" {
				field.Comment = "; // " + f.Comment
			} else {
				field.Comment = ";"
			}
		}
		message.Name = tableName + "Request"
		message.Fields = append(message.Fields, field)
	}
	s.Message = message
}

// MySQL 类型转 PB 类型
func TypeSqlToPb(m string) string {
	if _, ok := typeArr[m]; ok {
		return typeArr[m]
	}
	return "string"
}

var typeArr = map[string]string{
	"varchar":   "string",
	"text":      "string",
	"enum":      "int32",
	"tinyint":   "int32",
	"smallint":  "int32",
	"mediumint": "int32",
	"int":       "int32",
	"bigint":    "int64",
	"float":     "float",
	"double":    "double",
	"decimal":   "double",
	"timestamp": "string",
	"date":      "string",
	"datetime":  "string",
	"blob":      "blob",
}

// 下划线转大驼峰
func UpperCamel(str string) string {
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

// 大驼峰转横杠分割
func Kebab(str string) string {
	var result strings.Builder
	for i, char := range str {
		if i > 0 && unicode.IsUpper(char) {
			result.WriteRune('-')
		}
		result.WriteRune(unicode.ToLower(char))
	}
	return result.String()
}

// 判断文件是否存在
func IsFile(f string) bool {
	fi, e := os.Stat(f)
	if e != nil {
		return false
	}
	return !fi.IsDir()
}

type Database struct {
	Name     string
	Comments map[string]string
	Tables   map[string][]Column
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
	Name     string
	Function Function
	Message  Message
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
	Fields []Field
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
