syntax = "proto3";

package {{.Models}};

option go_package = "{{.Models}}";

import "google/protobuf/timestamp.proto";

// The {{.Models}} service definition.
service {{.Name}} {
 {{range .Funcs}} rpc {{.Name}}({{.RequestName}}) returns ({{.ResponseName}}) {}
 {{end}}
}
{{range .MessageList}}
message {{.Name}} {
{{range .MessageDetail}} {{.TypeName}} {{.AttrName}} = {{.Num}}{{.Comment}}
{{end}}}
{{end}}

