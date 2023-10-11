syntax = "proto3";

import "google/api/annotations.proto";

package {{.Package}};
option go_package = "/{{.Package}}";

{{range .Services}}
// The {{.Name}} service definition.
service {{.Name}} {
	rpc {{.Function.Name}}({{.Function.RequestName}}) returns ({{.Function.ResponseName}}) {
		option (google.api.http) = {
			{{.Function.Method}}: "/{{.Function.Path}}"
		};
	}
}

message {{.Message.Name}} {
{{range .Message.Fields}} {{.Type}} {{.Name}} = {{.Num}}{{.Comment}}
{{end}}}
{{end}}
