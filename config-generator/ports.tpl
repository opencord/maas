"ports": {
{{ range . }}
  "{{ .Location.ElementID }}/{{ .Location.Port }}" : {
    	"interfaces": [
    		{
        		"ips" : [ "{{ .Gateway }}254/24" ]
    		}
    	]
	}{{ .Comma }}
{{ end }}
},

