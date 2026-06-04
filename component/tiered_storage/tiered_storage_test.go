package tiered_storage

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/loopback"
	"github.com/Seagate/cloudfuse/internal"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var home_dir, _ = os.UserHomeDir()

type tieredStorageTestSuite struct {
	suite.Suite
	assert            *assert.Assertions
	tieredStorage     *TieredStorage
	loopback          internal.Component
	cache_path        string // uses os.Separator (filepath.Join)
	fake_storage_path string // uses os.Separator (filepath.Join)
	useMock           bool
	mockCtrl          *gomock.Controller
	mock              *internal.MockComponent
}

func newLoopbackFS(cachePath string) internal.Component {
	loopback := loopback.NewLoopbackFSComponent()
	_ = loopback.Configure(true)
	return loopback
}

func newTestTieredStorage(next internal.Component) *TieredStorage {

	tieredStorage := NewTieredStorageComponent()
	tieredStorage.SetNextComponent(next)
	err := tieredStorage.Configure(true)
	if err != nil {
		panic(fmt.Sprintf("Unable to configure tiered storage: %v", err))
	}
	return tieredStorage.(*TieredStorage)
}

func randomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)[:length]
}

func (suite *tieredStorageTestSuite) SetupTest() {
	err := log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_DEBUG()})
	if err != nil {
		panic(fmt.Sprintf("Unable to set silent logger as default: %v", err))
	}
	rand := randomString(8)
	suite.cache_path = filepath.Join(home_dir, "file_cache"+rand)
	suite.fake_storage_path = filepath.Join(home_dir, "fake_storage"+rand)
	defaultConfig := fmt.Sprintf(
		"file_cache:\n  path: %s\n  offload-io: true\n\nloopbackfs:\n  path: %s",
		suite.cache_path,
		suite.fake_storage_path,
	)
	suite.useMock = false
	log.Debug("%s", defaultConfig)

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	if err != nil {
		fmt.Printf(
			"fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.cache_path,
			err,
		)
	}
	err = os.RemoveAll(suite.fake_storage_path)
	if err != nil {
		fmt.Printf(
			"fileCacheTestSuite::SetupTest : os.RemoveAll(%s) failed [%v]\n",
			suite.fake_storage_path,
			err,
		)
	}
	suite.setupTestHelper(defaultConfig)
}

func (suite *tieredStorageTestSuite) setupTestHelper(configuration string) {
	suite.assert = assert.New(suite.T())

	err := config.ReadConfigFromReader(strings.NewReader(configuration))
	suite.assert.NoError(err)
	if suite.useMock {
		suite.mockCtrl = gomock.NewController(suite.T())
		suite.mock = internal.NewMockComponent(suite.mockCtrl)
		suite.tieredStorage = newTestTieredStorage(suite.mock)
		// always simulate being offline
		suite.mock.EXPECT().CloudConnected().AnyTimes().Return(false)
	} else {
		suite.loopback = newLoopbackFS(suite.fake_storage_path)
		suite.tieredStorage = newTestTieredStorage(suite.loopback)
		err = suite.loopback.Start(context.Background())
		suite.assert.NoError(err)
	}
	err = suite.tieredStorage.Start(context.Background())
	if err != nil {
		panic(fmt.Sprintf("Unable to start tiered storage [%s]", err.Error()))
	}

}

func (suite *tieredStorageTestSuite) cleanupTest() {
	err := suite.tieredStorage.Stop()
	if err != nil {
		panic(fmt.Sprintf("Unable to stop tiered storage [%s]", err.Error()))
	}
	if suite.useMock {
		suite.mockCtrl.Finish()
	} else {
		err = suite.loopback.Stop()
		suite.assert.NoError(err)
	}

	// Delete the temp directories created
	err = os.RemoveAll(suite.cache_path)
	suite.assert.NoError(err)
	err = os.RemoveAll(suite.fake_storage_path)
	suite.assert.NoError(err)
}

func (suite *tieredStorageTestSuite) TestOpenFileNotInCache() {
	defer suite.cleanupTest()
	path := "file7"

	//put file in cloud
	handle, _ := suite.loopback.CreateFile(internal.CreateFileOptions{Name: path, Mode: 0777})
	testData := "test data"
	data := []byte(testData)
	_, err := suite.loopback.WriteFile(
		&internal.WriteFileOptions{Handle: handle, Offset: 0, Data: data},
	)
	suite.assert.NoError(err)
	err = suite.loopback.ReleaseFile(internal.ReleaseFileOptions{Handle: handle})
	suite.assert.NoError(err)

	//open file through tiered storage, should succeed and return a handle with correct path
	handle, err = suite.tieredStorage.OpenFile(
		internal.OpenFileOptions{
			Name:  path,
			Flags: os.O_RDWR,
			Mode:  0666, //random mode, since we didn't do the other stuff yet
		},
	)
	suite.assert.NoError(err)
	suite.assert.Equal(path, handle.Path)

	// Verify it was now downloaded to the local tiered storage cache
	suite.assert.FileExists(filepath.Join(suite.cache_path, path))
}
func TestTieredStorageTestSuite(t *testing.T) {
	suite.Run(t, new(tieredStorageTestSuite))
}
