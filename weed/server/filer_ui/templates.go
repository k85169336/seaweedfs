package master_ui

import (
	"html/template"
)

var StatusTpl = template.Must(template.New("status").Parse(`<!DOCTYPE html>
<html>
<head>
	<title>SeaweedFS Filer</title>
	<link rel="icon" href="http://7viirv.com1.z0.glb.clouddn.com/seaweed_favicon.png" sizes="32x32" />
	<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.1/css/bootstrap.min.css">
</head>
<body>
	<div class="container">
		<div class="page-header">
			<h1>
				<img src="http://7viirv.com1.z0.glb.clouddn.com/seaweed50x50.png"></img>
				SeaweedFS Filer
			</h1>
		</div>
		<div class="row">
			{{.Path}}
		</div>

		<div class="row">
			<ul>
				{{$path := .Path }}
				{{ range $dirs_index, $dir := .Directories }}
				<li>
					<img src="https://www.w3.org/TR/WWWicn/folder.gif" width="20" height="23">
					<a href={{ print $path  $dir  "/"}} >
						{{ $dir }}
					</a>
				</li>
				{{ end }}

				{{ range $file_index, $file := .Files }}
				<li>
					<a href={{ print $path  $file.Name}} >
					{{ $file.Name }}
					</a>
				</li>
				{{ end }}
			</ul>
		</div>

		{{if .ShouldDisplayLoadMore}}
		<div class="row">
		<a href={{ print .Path "?limit=" .Limit	"&lastFileName=" .LastFileName}} >
		Load more
		</a>
		</div>
		{{end}}
	</div>
</body>
</html>
`))
