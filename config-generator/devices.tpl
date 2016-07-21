{
 "devices" : {
{{ range $index, $element := . }}
    "{{ .Id}}" : {
            "segmentrouting" : {
                "name" : "device-{{ .ChassisId }}",
                "nodeSid" : 10{{ $index }},
                "routerIp" : "{{ .Annotations.ManagementAddress }}",
                "routerMac" : "cc:37:ab:00:00:0{{ $index }}",
                "isEdgeRouter" : true,
                "adjacencySids" : []
            }
    }{{ .Comma }}{{ end }}
 },

