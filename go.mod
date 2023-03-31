module lyvecloudfuse

go 1.16

require (
	cloud.google.com/go/storage v1.30.1 // indirect
	github.com/Azure/azure-pipeline-go v0.2.4-0.20220425205405-09e6f201e1e4
	github.com/Azure/azure-storage-azcopy/v10 v10.17.0
	github.com/Azure/azure-storage-blob-go v0.15.0
	github.com/Azure/go-autorest/autorest v0.11.28
	github.com/Azure/go-autorest/autorest/adal v0.9.23
	github.com/JeffreyRichter/enum v0.0.0-20180725232043-2567042f9cda
	github.com/aws/aws-sdk-go-v2 v1.17.7
	github.com/aws/aws-sdk-go-v2/config v1.18.19
	github.com/aws/aws-sdk-go-v2/credentials v1.13.18
	github.com/aws/aws-sdk-go-v2/service/s3 v1.31.0
	github.com/aws/smithy-go v1.13.5
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/fsnotify/fsnotify v1.6.0
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/golang/mock v1.6.0
	github.com/googleapis/gax-go/v2 v2.8.0 // indirect
	github.com/hillu/go-ntdll v0.0.0-20230314165016-3d2a6125cd5d // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/mattn/go-ieproxy v0.0.10 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/montanaflynn/stats v0.6.6
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58
	github.com/pelletier/go-toml/v2 v2.0.7 // indirect
	github.com/pkg/xattr v0.4.9 // indirect
	github.com/radovskyb/watcher v1.0.7
	github.com/sevlyar/go-daemon v0.1.6
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cobra v1.6.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.15.0
	github.com/stretchr/testify v1.8.2
	github.com/winfsp/cgofuse v1.5.0
	go.uber.org/atomic v1.10.0
	golang.org/x/crypto v0.7.0 // indirect
	golang.org/x/sys v0.6.0
	google.golang.org/genproto v0.0.0-20230331144136-dcfb400f0633 // indirect
	gopkg.in/ini.v1 v1.67.0
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
)

replace github.com/spf13/cobra => github.com/gapra-msft/cobra v1.4.1-0.20220411185530-5b83e8ba06dd
