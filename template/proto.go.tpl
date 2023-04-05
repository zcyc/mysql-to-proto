syntax = "proto3";

import "google/protobuf/timestamp.proto";
import "google/api/annotations.proto";

package {{.Package}};
option go_package = "/{{.Package}}";

// The {{.Name}} service definition.
service {{.Name}} {
 {{range .Functions}} rpc {{.Name}}({{.RequestName}}) returns ({{.ResponseName}}) {
    option (google.api.http) = {
			{{.Method}}: "/v1/{{.Path}}"
	};

 }
 {{end}}
}
{{range .Messages}}
message {{.Name}} {
{{range .Detail}} {{.Type}} {{.Name}} = {{.Num}}{{.Comment}}
{{end}}}
{{end}}

