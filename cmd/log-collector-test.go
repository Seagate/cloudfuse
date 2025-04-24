package cmd

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

)

type logCollectTestSuite struct {
	suite.Suite
	assert *assert.Assertions
}

func (suite *logCollectTestSuite) SetupTest {
	suite.assert = assert.New(suite.T())
}

func (suite *logCollectTestSuite) cleanupTest {
	resetCLIFlags(*gatherLogsCmd)
}


