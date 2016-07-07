"hosts" : {
	{{ range . }}
	"{{ .Mac }}/-1" : {
		"basic" : {
			"ips" : ["{{ range $element := .IpAddresses }}{{ $element }}{{ end}}"],
			"location" : "{{ .Location.ElementID }}/{{ .Location.Port }}"
		}
	}{{ .Comma }}
	{{ end }}
  },

  "apps" : {
        "org.onosproject.core" : {
            "core" : {
                "linkDiscoveryMode" : "STRICT" 
            }    
        }
    }

}