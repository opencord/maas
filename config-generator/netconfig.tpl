{
    "devices": {
        {{ range $index, $element := .Devices }}{{ if $index }},
        {{ end }}"{{ .Id }}": {
            "segmentrouting": {
                "name": "device-{{ .ChassisId }}",
                "ipv4NodeSid": {{ add 100 $index }},
                "ipv4Loopback": "{{ .Annotations.ManagementAddress }}",
                "routerMac": "{{ .Mac }}",
                "isEdgeRouter": {{ .IsEdgeRouter }},
                "adjacencySids": []
            }
        }{{ end }}
    },
    "ports": {
        {{ range $index, $element := .Hosts }}{{ if $index }},
        {{ end }}"{{ .Location.ElementID }}/{{ .Location.Port }}": {
            "interfaces": [
                {
                    "ips": [ "{{ gateway .IpAddresses }}" ],
                    "vlan-untagged" : {{ vlan .IpAddresses }}
                }
            ]
        }{{ end }}
    },
    "apps" : {
        "org.onosproject.segmentrouting" : {
            "segmentrouting" : {
                "vRouterMacs" : [ "a4:23:05:06:01:01" ]
            }
        }
    }
}
