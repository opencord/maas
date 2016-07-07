 "links": {
     {{ range . }}
     "{{ .Val }}": {
         "basic": {}
     }{{ .Comma }}
     {{ end }}
 },

