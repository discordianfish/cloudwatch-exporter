module github.com/discordianfish/cloudwatch-exporter

go 1.15

require (
	github.com/aws/aws-sdk-go v1.27.0
	github.com/aws/aws-sdk-go-v2 v1.2.1
	github.com/aws/aws-sdk-go-v2/config v1.1.2
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.1.2
	github.com/go-kit/kit v0.10.0
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.4
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.19.0
	github.com/prometheus/exporter-toolkit v0.5.1
	github.com/stoewer/go-strcase v1.2.0
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)
