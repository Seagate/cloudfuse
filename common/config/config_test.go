/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package config

import (
	"os"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/awnumar/memguard"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type Labels struct {
	App string `config:"app"`
}

type Metadata struct {
	Name  string `config:"name"`
	Label Labels `config:"labels"`
}

type MatchLabels struct {
	App string `config:"app"`
}

type Selector struct {
	Match MatchLabels `config:"matchLabels"`
}

type Template struct {
	Meta Metadata `config:"metadata"`
}

type Spec struct {
	Replicas int32    `config:"replicas"`
	Select   Selector `config:"selector"`
	Templ    Template `config:"template"`
}

type Config1 struct {
	ApiVer string   `config:"apiVersion"`
	Kind   string   `config:"kind"`
	Meta   Metadata `config:"metadata"`
}

type Config2 struct {
	ApiVer string   `config:"apiVersion"`
	Kind   string   `config:"kind"`
	Meta   Metadata `config:"metadata"`
	Specs  Spec     `config:"spec"`
}

type ConfigTestSuite struct {
	suite.Suite
}

var config1 = `
apiVersion: v1
kind: Pod
metadata:
  name: rss-site
  labels:
    app: web
`

var config2 = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rss-site
  labels:
    app: web
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
`

var metaconf = `
name: hooli
labels:
  app: pied-piper
`

// Function to test only config file reader: testcase 2
func (suite *ConfigTestSuite) TestPlainConfig2Reader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	err := ReadConfigFromReader(strings.NewReader(config2))
	assert.NoError(err)

	//Case 1
	metaDeepOpts2 := &Metadata{}
	metaDeepOpts2Truth := &Metadata{
		Label: Labels{
			App: "web",
		},
	}
	err = UnmarshalKey("spec.template.metadata", metaDeepOpts2)
	assert.NoError(err)
	assert.Equal(metaDeepOpts2Truth, metaDeepOpts2)

	//Case 2
	templatOpts2 := &Template{}
	templatOpts2Truth := &Template{
		Meta: Metadata{
			Label: Labels{
				App: "web",
			},
		},
	}
	err = UnmarshalKey("spec.template", templatOpts2)
	assert.NoError(err)
	assert.Equal(templatOpts2Truth, templatOpts2)

	//Case 3
	specOpts2 := &Spec{}
	specOpts2Truth := &Spec{
		Replicas: 2,
		Select: Selector{
			Match: MatchLabels{
				App: "web",
			},
		},
		Templ: Template{
			Meta: Metadata{
				Label: Labels{
					App: "web",
				},
			},
		},
	}
	err = UnmarshalKey("spec", specOpts2)
	assert.NoError(err)
	assert.Equal(specOpts2Truth, specOpts2)

	// Case 4
	opts2 := &Config2{}
	opts2Truth := &Config2{
		ApiVer: "apps/v1",
		Kind:   "Deployment",
		Meta: Metadata{
			Name: "rss-site",
			Label: Labels{
				App: "web",
			},
		},
		Specs: Spec{
			Replicas: 2,
			Select: Selector{
				Match: MatchLabels{
					App: "web",
				},
			},
			Templ: Template{
				Meta: Metadata{
					Label: Labels{
						App: "web",
					},
				},
			},
		},
	}
	err = Unmarshal(opts2)
	assert.NoError(err)
	assert.Equal(opts2Truth, opts2)

	//Case 5
	apiVersion := 0
	err = UnmarshalKey("apiVersion", &apiVersion)
	assert.Error(err)
}

// Function to test only config file reader: testcase 1
func (suite *ConfigTestSuite) TestPlainConfig1Reader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	err := ReadConfigFromReader(strings.NewReader(config1))
	assert.NoError(err)

	//Case1
	opts1 := &Config1{}
	opts1Truth := &Config1{
		ApiVer: "v1",
		Kind:   "Pod",
		Meta: Metadata{
			Name: "rss-site",
			Label: Labels{
				App: "web",
			},
		},
	}
	err = Unmarshal(opts1)
	assert.NoError(err)
	assert.Equal(opts1Truth, opts1)

	//Case2
	metaOpts1 := &Metadata{}
	metaOpts1Truth := &Metadata{
		Name: "rss-site",
		Label: Labels{
			App: "web",
		},
	}
	err = UnmarshalKey("metadata", metaOpts1)
	assert.NoError(err)
	assert.Equal(metaOpts1Truth, metaOpts1)

	//Case 3
	labelOpts1 := &Labels{}
	labelOpts1Truth := &Labels{
		App: "web",
	}
	err = UnmarshalKey("metadata.labels", labelOpts1)
	assert.NoError(err)
	assert.Equal(labelOpts1Truth, labelOpts1)

	//Case 4:
	randOpts := struct {
		NewName       string `config:"newname"`
		NotExistField int    `config:"notexists"`
	}{}

	err = Unmarshal(&randOpts)
	assert.NoError(err)
	assert.Empty(randOpts)
}

// Function to test config reader when there is environment variables that shadow config file
func (suite *ConfigTestSuite) TestEnvShadowedConfigReader() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())
	metaOptsTruth := &Metadata{
		Name: "mcdhee",
		Label: Labels{
			App: "zigby",
		},
	}
	err := os.Setenv("CF_TEST_NAME", metaOptsTruth.Name)
	assert.NoError(err)
	err = os.Setenv("CF_TEST_APP", metaOptsTruth.Label.App)
	assert.NoError(err)

	//Case 1
	BindEnv("name", "CF_TEST_NAME")
	BindEnv("labels.app", "CF_TEST_APP")

	metaOpts := &Metadata{}
	err = Unmarshal(metaOpts)
	assert.NoError(err)
	assert.Equal(metaOptsTruth, metaOpts)

	ResetConfig()

	//Case 2
	err = ReadConfigFromReader(strings.NewReader(metaconf))
	assert.NoError(err)
	BindEnv("name", "CF_TEST_NAME")
	BindEnv("labels.app", "CF_TEST_APP")
	metaOpts = &Metadata{}
	err = Unmarshal(metaOpts)
	assert.NoError(err)
	assert.Equal(metaOptsTruth, metaOpts)

}

func (suite *ConfigTestSuite) TestConfigFileDescription() {
	defer suite.cleanupTest()
	assert := assert.New(suite.T())

	_ = os.WriteFile("test.yaml", []byte(config2), 0644)
	plaintext, err := os.ReadFile("test.yaml")
	assert.NoError(err)
	assert.NotNil(plaintext)

	encryptedPassphrase := memguard.NewEnclave([]byte("12312312312312312312312312312312"))

	cipherText, err := common.EncryptData(plaintext, encryptedPassphrase)
	assert.NoError(err)
	err = os.WriteFile("test_enc.yaml", cipherText, 0644)
	assert.NoError(err)

	err = DecryptConfigFile("test_enc.yaml", encryptedPassphrase)
	assert.NoError(err)

	_ = os.Remove("test.yaml")
	_ = os.Remove("test_enc.yaml")
}

func (suite *ConfigTestSuite) cleanupTest() {
	ResetConfig()
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}
