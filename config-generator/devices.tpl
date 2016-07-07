{
 "devices" : {
    {{ range . }}
        "{{ .Id}}" : {
            "segmentrouting" : {
                "name" : "device-{{ .ChassisId }}",
                "nodeSid" : 1{{ .ChassisId }},
                "routerIp" : "{{ .Annotations.ManagementAddress }}",
                "routerMac" : "cc:37:ab:cc:ef:{{ .ChassisId }}",
                "isEdgeRouter" : true,
                "adjacencySids" : []
            }
    }{{ .Comma }}
    {{ end }}

},

