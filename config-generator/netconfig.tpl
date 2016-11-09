{
    "devices": {
        {{ range $index, $element := .Devices }}{{ if $index }},
        {{ end }}"{{ .Id }}": {
            "segmentrouting": {
                "name": "device-{{ .ChassisId }}",
                "nodeSid": {{ add 100 $index }},
                "routerIp": "{{ .Annotations.ManagementAddress }}",
                "routerMac": "{{ .Mac }}",
                "isEdgeRouter": {{ .IsEdgeRouter }},
                "adjacencySids": []
            }
        }{{ end }}
    },
    "hosts": {
        {{ range $index, $element := .Hosts }}{{ if $index }},
        {{ end }}"{{ .Mac }}": {
            "ips": ["{{ range $ip := .IpAddresses }}{{ $ip }}{{ end }}"],
            "location": "{{ .Location.ElementID }}/{{ .Location.Port }}"
        }{{ end }}
    },
    "ports": {
        {{ range $index, $element := .Hosts }}{{ if $index }},
        {{ end }}"{{ .Location.ElementID }}/{{ .Location.Port }}": {
            "interfaces": [
                {
                    "ips": [ "{{ gateway .IpAddresses }}" ]
                }
            ]
        }{{ end }}
    },
    "links": {},
    "apps": {}
}
