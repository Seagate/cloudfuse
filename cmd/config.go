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

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Seagate/cloudfuse/common"
	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/component/attr_cache"
	"github.com/Seagate/cloudfuse/component/azstorage"
	"github.com/Seagate/cloudfuse/component/file_cache"
	"github.com/Seagate/cloudfuse/component/libfuse"
	"github.com/awnumar/memguard"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

// Top-level struct to hold application context, including tview application instance,
// page stack, user configuration data, and UI theme settings.
type appContext struct {
	app    *tview.Application
	pages  *tview.Pages
	config *userConfig
	theme  *uiTheme
}

// Struct to hold user configuration data collected from the TUI session.
type userConfig struct {
	configEncryptionPassphrase string   // Sets config file encryption passphrase
	configFilePath             string   // Sets file_cache.path
	accountName                string   // Sets azstorage.account-name
	accountKey                 string   // Sets azstorage.account-key
	accessKey                  string   // Sets s3storage.key-id
	secretKey                  string   // Sets s3storage.secret-key
	containerName              string   // Sets azstorage.container-name
	bucketName                 string   // Sets s3storage.bucket-name
	endpointURL                string   // Sets s3storage.endpoint
	bucketList                 []string // Holds list of available buckets retrieved from cloud provider (for s3 only).
	storageProtocol            string   // Sets 's3storage' or 'azstorage' based on selected provider
	storageProvider            string   // Options: 'LyveCloud', 'Microsoft', 'AWS', or 'Other (s3)'. Used to set certain UI elements.
	cacheMode                  string   // Sets 'components' to include 'file_cache' or 'block_cache'
	enableCaching              bool     // If true, sets cacheMode to file_cache. If false, block_cache
	cacheLocation              string   // Sets file_cache.path @ startup to default: $HOME/.cloudfuse/cache
	cacheSize                  string   // User-defined cache size as %
	availableCacheSizeGB       int      // Total available cache size in GB @ the cache location
	currentCacheSizeGB         int      // Current cache size in GB based on 'cacheSize' percentage
	clearCacheOnStart          bool     // If false, sets 'allow-non-empty-temp' to true
	cacheRetentionDuration     int      // User-defined cache retention duration. Default is '2'
	cacheRetentionUnit         string   // User-defined cache retention unit (sec, min, hours, days). Default is 'days'
	cacheRetentionDurationSec  int      // Sets 'file_cache.timeout-sec' from 'cacheRetentionDuration'
}

// Struct to hold UI theme settings, including colors and labels for various widgets.
type uiTheme struct {
	widgetLabelColor           tcell.Color
	widgetFieldBackgroundColor tcell.Color
	navigationButtonColor      tcell.Color
	navigationButtonTextColor  tcell.Color
	navigationStartLabel       string
	navigationHomeLabel        string
	navigationNextLabel        string
	navigationBackLabel        string
	navigationPreviewLabel     string
	navigationQuitLabel        string
	navigationFinishLabel      string
	navigationWidgetHeight     int
}

// Global general purpose vars
var (
	colorYellow tcell.Color = tcell.GetColor("#FFD700")
	colorGreen  tcell.Color = tcell.GetColor("#6EBE49")
	colorBlack  tcell.Color = tcell.ColorBlack
)

// Struct to hold the final configuration data to be written to the YAML config file.
type configOptions struct {
	Components []string                    `yaml:"components,omitempty"`
	Libfuse    libfuse.LibfuseOptions      `yaml:"libfuse,omitempty"`
	FileCache  file_cache.FileCacheOptions `yaml:"file_cache,omitempty"`
	AttrCache  attr_cache.AttrCacheOptions `yaml:"attr_cache,omitempty"`
	S3Storage  s3StorageConfig             `yaml:"s3storage,omitempty"`
	AzStorage  azstorage.AzStorageOptions  `yaml:"azstorage,omitempty"`
}

// Struct to hold s3 storage configuration options for the YAML config file.
// TODO: change to using s3storage.Options from component/s3storage/config.go
type s3StorageConfig struct {
	BucketName      string `yaml:"bucket-name,omitempty"`
	KeyID           string `yaml:"key-id"`
	SecretKey       string `yaml:"secret-key"`
	Endpoint        string `yaml:"endpoint"`
	EnableDirMarker bool   `yaml:"enable-dir-marker"`
}

// Constructor for appContext struct. Initializes default values for userConfig and uiTheme.
func newAppContext() *appContext {
	return &appContext{
		app:   tview.NewApplication(),
		pages: tview.NewPages(),
		config: &userConfig{
			enableCaching:          true,
			cacheLocation:          getDefaultCachePath(),
			cacheSize:              "80",
			cacheRetentionDuration: 2,
			clearCacheOnStart:      false,
		},
		theme: &uiTheme{
			widgetLabelColor:           colorYellow,
			widgetFieldBackgroundColor: colorYellow,
			navigationButtonColor:      colorGreen,
			navigationButtonTextColor:  colorBlack,
			navigationStartLabel:       "[black]🚀 Start[-]",
			navigationHomeLabel:        "[black]🏠 Home[-]",
			navigationNextLabel:        "[black]🡲  Next[-]",
			navigationBackLabel:        "[black]🡰  Back[-]",
			navigationPreviewLabel:     "[black]📄 Preview[-]",
			navigationQuitLabel:        "[black]❌ Quit[-]",
			navigationFinishLabel:      "[black]✅ Finish[-]",
			navigationWidgetHeight:     3,
		},
	}
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Launch the interactive configuration tool.",
	Long:  "Starts an interactive terminal-based UI to generate your Cloudfuse configuration file.",
	RunE: func(cmd *cobra.Command, args []string) error {
		tui := newAppContext()
		if err := tui.run(); err != nil {
			return fmt.Errorf("Failed to run TUI: %v", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

// Main function to run the TUI application.
// Initializes the tview application, builds the TUI application, and runs it.
func (tui *appContext) run() error {
	// Disable cloudfuse logging during TUI session to prevent log messages from interfering with the UI.
	log.SetDefaultLogger("silent", common.LogConfig{Level: common.ELogLevel.LOG_OFF()})

	tui.app.EnableMouse(true)
	tui.app.EnablePaste(true)

	tui.build()

	// Run the application
	if err := tui.app.Run(); err != nil {
		panic(err)
	}

	return nil
}

// Function to build the TUI application. Initializes the pages and adds them to the page stack.
func (tui *appContext) build() {

	// Initialize the pages
	homePage := tui.buildHomePage()         // --- Home Page ---
	page1 := tui.buildStorageProviderPage() // --- Page 1: Storage Provider Selection ---
	page2 := tui.buildEndpointURLPage()     // --- Page 2: Endpoint URL Entry ---
	page3 := tui.buildCredentialsPage()     // --- Page 3: Credentials Entry ---
	page4 := tui.buildBucketSelectionPage() // --- Page 4: Bucket Selection ---
	page5 := tui.buildCachingPage()         // --- Page 5: Caching Settings ---

	// Add pages to the page stack
	tui.pages.AddPage("home", homePage, true, true)
	tui.pages.AddPage("page1", page1, true, false)
	tui.pages.AddPage("page2", page2, true, false)
	tui.pages.AddPage("page3", page3, true, false)
	tui.pages.AddPage("page4", page4, true, false)
	tui.pages.AddPage("page5", page5, true, false)

	tui.app.SetRoot(tui.pages, true)
}

//	--- Page 0: Home Page ---
//
// Function to build the home page of the TUI application. Displays a
// welcome banner, instructions, and buttons to start or quit the application.
func (tui *appContext) buildHomePage() tview.Primitive {
	bannerText := fmt.Sprintf(
		"[%s::b]"+
			" █▀▀░█░░░█▀█░█░█░█▀▄░█▀▀░█░█░█▀▀░█▀▀\n"+
			"░█░░░█░░░█░█░█░█░█░█░█▀▀░█░█░▀▀█░█▀▀\n"+
			"░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀░░░▀▀▀░▀▀▀░▀▀▀[-]\n\n"+
			"[white::b]Welcome to the CloudFuse Configuration Tool\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n"+
			"[%s::b]Cloud storage configuration made easy via terminal.[-]\n\n"+
			"[::b]Press [%s]Start[-] to begin or [red]Quit[-] to exit.\n",
		colorGreen, colorYellow, colorGreen, colorYellow)

	// Banner text widget
	bannerTextWidget := tview.NewTextView().
		SetText(centerText(bannerText, 75)).
		SetDynamicColors(true).
		SetWrap(true)

	instructionsText := fmt.Sprintf(
		"[%s::b]Instructions:[::-]\n"+
			"[%s::b] •[-::-] Use your mouse or arrow keys to navigate.\n"+
			"[%s::b] •[-::-] Press Enter or left-click to select items.\n"+
			"[%s::b] •[-::-] [::]For the best experience, expand terminal window to full size.\n",
		colorYellow, colorGreen, colorGreen, colorGreen)

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetDynamicColors(true).
		SetWrap(true)

	// Start/Quit buttons widget
	startQuitButtonsWidget := tview.NewForm().
		AddButton(tui.theme.navigationStartLabel, func() {
			tui.pages.SwitchToPage("page1")
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetButtonTextColor(tui.theme.navigationButtonTextColor)

	aboutText := fmt.Sprintf(
		"[%s::b]ABOUT[-::-]\n"+
			"[white]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n"+
			"[grey::i]CloudFuse TUI Configuration Tool\n"+
			"Seagate Technology, LLC\n"+
			"cloudfuse@seagate.com\n"+
			"Version: %s",
		colorYellow, common.CloudfuseVersion)

	// About text widget
	aboutTextWidget := tview.NewTextView().
		SetText(centerText(aboutText, 75)).
		SetDynamicColors(true).
		SetWrap(true)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(bannerTextWidget, getTextHeight(bannerText), 0, false). // Banner Widget
		AddItem(nil, 1, 0, false).                                      // Padding
		AddItem(startQuitButtonsWidget, 3, 0, false).                   // Start/Quit buttons widget
		AddItem(nil, 1, 0, false).                                      // Padding
		AddItem(instructionsTextWidget, 4, 0, false).                   // Instructions widget
		AddItem(nil, 2, 0, false).                                      // Padding
		AddItem(aboutTextWidget, 9, 0, false).                          // About widget
		AddItem(nil, 1, 0, false)                                       // Bottom padding

	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 1: Storage Provider Selection ---
//
// Function to build the storage provider selection page. Allows users to select their cloud storage provider
// from a dropdown list. The options are: LyveCloud, Microsoft, AWS, and Other S3.
func (tui *appContext) buildStorageProviderPage() tview.Primitive {
	instructionsText := fmt.Sprintf(
		"[%s::b] Select Your Cloud Storage Provider[-::-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n"+
			"[white::b] Choose your cloud storage provider from the dropdown below.[-::-]\n"+
			"[grey::i] If your provider is not listed, choose [darkmagenta::b]Other (s3)[-::-][grey::i]. You’ll be\n"+
			" prompted to enter the endpoint URL and region manually.[-::-]\n",
		colorGreen, colorYellow)

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetDynamicColors(true).
		SetWrap(true)

	// Dropdown widget for selecting storage provider
	storageProviderDropdownWidget := tview.NewDropDown().
		SetLabel("📦 Storage Provider: ").
		SetOptions([]string{" LyveCloud ⬇️", " Microsoft ", " AWS ", " Other (s3) "}, func(option string, index int) {
			tui.config.storageProvider = option
			switch option {
			case " LyveCloud ⬇️":
				tui.config.storageProtocol = "s3storage"
				tui.config.storageProvider = "LyveCloud"
			case " Microsoft ":
				tui.config.storageProtocol = "azstorage"
				tui.config.storageProvider = "Microsoft"
			case " AWS ":
				tui.config.storageProtocol = "s3storage"
				tui.config.storageProvider = "AWS"
			case " Other (s3) ":
				tui.config.storageProtocol = "s3storage"
				tui.config.storageProvider = "Other"
				tui.config.endpointURL = ""
			default:
				tui.config.storageProtocol = "s3storage"
				tui.config.storageProvider = "LyveCloud"
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack).
		SetFieldWidth(14)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(tui.theme.navigationHomeLabel, func() {
			tui.pages.SwitchToPage("home")
		}).
		AddButton(tui.theme.navigationNextLabel, func() {
			// If Microsoft is selected, switch to page 3 and skip endpoint entry, handled internally by Azure SDK.
			if tui.config.storageProvider == "Microsoft" {
				page3 := tui.buildCredentialsPage()
				tui.pages.AddPage("page3", page3, true, false)
				tui.pages.SwitchToPage("page3")
			} else {
				page2 := tui.buildEndpointURLPage()
				tui.pages.AddPage("page2", page2, true, false)
				tui.pages.SwitchToPage("page2")
			}
		}).
		AddButton(tui.theme.navigationPreviewLabel, func() {
			previewPage := tui.buildPreviewPage("page1")
			tui.pages.AddAndSwitchToPage("previewPage", previewPage, true)
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetButtonTextColor(tui.theme.navigationButtonTextColor)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(storageProviderDropdownWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, tui.theme.navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 2: Endpoint URL Entry Page ---
//
// Function to build the endpoint URL page. Allows users to enter the endpoint URL for their cloud storage provider.
// It validates the endpoint URL format and provides help text based on the selected provider.
func (tui *appContext) buildEndpointURLPage() tview.Primitive {
	var urlRegionHelpText string

	// Determine URL help text based on selected provider
	switch tui.config.storageProvider {
	case "LyveCloud":
		urlRegionHelpText = "[::b] You selected LyveCloud as your storage provider.[::-]\n\n" +
			" For LyveCloud, the endpoint URL format is generally:\n" +
			"[darkmagenta::b] https://s3.<[darkcyan::b]region[darkmagenta::b]>.<[darkcyan::b]identifier[darkmagenta::b]>.lyve.seagate.com[-]\n\n" +
			"\t\t\t\t Example:\n [darkmagenta::b]https://s3.us-east-1.sv15.lyve.seagate.com[-]\n\n" +
			"[grey::i] *Refer to your LyveCloud portal for valid formats.[-::-]"

	case "AWS":
		urlRegionHelpText = "[::b] You selected AWS as your storage provider.[::-]\n\n" +
			" The endpoint URL format is generally:\n" +
			"[darkmagenta::b] https://s3.<[darkcyan::b]region[darkmagenta::b]>.amazonaws.com[-]\n\n" +
			"\t\t\t Example:\n[darkmagenta::b] https://s3.us-east-1.amazonaws.com[-]\n\n" +
			"[grey::i] *Refer to your AWS portal for valid formats.[-::-]"

	case "Other":
		urlRegionHelpText = "[::b] You selected a custom s3 provider.[::-]\n\n" +
			" Enter the endpoint URL.\n\n" +
			"[grey::i] *Refer to your provider’s documentation for valid formats.[-::-]"
	}

	instructionsText := fmt.Sprintf(
		"[%s::b] Enter Endpoint URL for %s[-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"+
			"[white]\n%s", colorGreen, tui.config.storageProvider, colorYellow, urlRegionHelpText)

	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetWrap(true).
		SetDynamicColors(true)

	endpointURLFieldWidget := tview.NewInputField().
		SetLabel("🔗 Endpoint URL: ").
		SetText(tui.config.endpointURL).
		SetFieldWidth(50).
		SetChangedFunc(func(url string) {
			tui.config.endpointURL = strings.TrimSpace(url)
		}).
		SetPlaceholder("\t\t\t\t<ENTER URL HERE>").
		SetPlaceholderTextColor(tcell.ColorGray).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(tui.theme.navigationHomeLabel, func() {
			tui.pages.SwitchToPage("home")
		}).
		AddButton(tui.theme.navigationNextLabel, func() {
			if err := tui.validateEndpointURL(tui.config.endpointURL); err != nil {
				tui.showErrorModal(
					fmt.Sprintf("[red::b]ERROR:[-::-] %s", err.Error()),
					func() {
						page2 := tui.buildEndpointURLPage()
						tui.pages.AddAndSwitchToPage("page2", page2, true)
					},
				)
				return
			}
			page3 := tui.buildCredentialsPage()
			tui.pages.AddAndSwitchToPage("page3", page3, true)
		}).
		AddButton(tui.theme.navigationBackLabel, func() {
			tui.pages.SwitchToPage("page1")
		}).
		AddButton(tui.theme.navigationPreviewLabel, func() {
			previewPage := tui.buildPreviewPage("page2")
			tui.pages.AddAndSwitchToPage("previewPage", previewPage, true)
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetButtonTextColor(tui.theme.navigationButtonTextColor)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(endpointURLFieldWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, tui.theme.navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 3: Credentials Page ---
//
// Function to build the credentials page. Allows users to enter their cloud storage credentials.
// If the storage protocol is "s3", it provides input fields for access key, secret key.
// If the storage protocol is "azure", it provides input fields for account name, account key, and container name.
func (tui *appContext) buildCredentialsPage() tview.Primitive {

	// Determine labels for input fields based on storage protocol.
	accessLabel := ""
	secretLabel := ""
	if tui.config.storageProtocol == "azstorage" {
		accessLabel = "🔑 Account Name: "
		secretLabel = "🔑 Account Key: "
	} else {
		accessLabel = "🔑 Access Key: "
		secretLabel = "🔑 Secret Key: "
	}

	instructionsText := fmt.Sprintf(
		"[%s::b] Enter Your Cloud Storage Credentials[-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[::-]\n\n"+
			"[%s::b] -%s[-::-] This is your unique identifier for accessing your cloud storage.\n"+
			"[%s::b] -%s[-::-] This is your secret password for accessing your cloud storage.\n",
		colorGreen, colorYellow, colorYellow, strings.Trim(accessLabel, "🔑 "),
		colorYellow, strings.Trim(secretLabel, "🔑 "),
	)

	if tui.config.storageProtocol == "azstorage" {
		instructionsText += fmt.Sprintf(
			"[%s::b] -Container Name:[-::-] This is the name of your Azure Blob Storage container.\n",
			colorYellow,
		)
	}

	instructionsText += fmt.Sprintf(
		"[%s::b] -Passphrase:[-::-] This is used to encrypt your configuration file.\n"+
			"\n[darkmagenta::i]\t\t\t*Keep these credentials secure. Do not share.[-]",
		colorYellow)

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Access key field widget
	accessKeyFieldWidget := tview.NewInputField().
		SetLabel(accessLabel).
		SetText(tui.config.accessKey).
		SetFieldWidth(50).
		SetChangedFunc(func(key string) {
			tui.config.accessKey = strings.TrimSpace(key)
			tui.config.accountName = strings.TrimSpace(key)
		}).
		SetPlaceholder("\t\t\t\t<ENTER KEY HERE>").
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Secret key field widget with masked input
	secretKeyFieldWidget := tview.NewInputField().
		SetLabel(secretLabel).
		SetText(string(tui.config.secretKey)).
		SetFieldWidth(50).
		SetChangedFunc(func(key string) {
			tui.config.secretKey = strings.TrimSpace(key)
			tui.config.accountKey = strings.TrimSpace(key)
		}).
		SetPlaceholder("\t\t\t\t<ENTER KEY HERE>").
		SetMaskCharacter('*').
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Container name field widget for Azure storage
	containerNameFieldWidget := tview.NewInputField().
		SetLabel("🪣 Container Name: ").
		SetText(tui.config.containerName).
		SetPlaceholder("\t\t\t\t<ENTER NAME HERE>").
		SetChangedFunc(func(name string) {
			tui.config.containerName = strings.TrimSpace(name)
			tui.config.bucketName = strings.TrimSpace(name)
		}).
		SetFieldWidth(50).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Passphrase field widget for config file encryption
	passphraseFieldWidget := tview.NewInputField().
		SetLabel("🔒 Passphrase: ").
		SetText(tui.config.configEncryptionPassphrase).
		SetFieldWidth(50).
		SetChangedFunc(func(passphrase string) {
			tui.config.configEncryptionPassphrase = strings.TrimSpace(passphrase)
		}).
		SetPlaceholder("\t\t\t <ENTER PASSPHRASE HERE>").
		SetMaskCharacter('*').
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(tui.theme.navigationHomeLabel, func() {
			tui.pages.SwitchToPage("home")
		}).
		AddButton(tui.theme.navigationNextLabel, func() {
			// TODO: Add validation for access key and secret key HERE
			// For now, just check that they are not empty
			if (tui.config.storageProtocol == "s3storage" && (len(tui.config.accessKey) == 0 || len(tui.config.secretKey) == 0)) ||
				(tui.config.storageProtocol == "azstorage" && (len(tui.config.accountName) == 0 || len(tui.config.accountKey) == 0 || len(tui.config.containerName) == 0)) ||
				len(tui.config.configEncryptionPassphrase) == 0 {
				tui.showErrorModal(
					"[red::b]ERROR:[-::-] Credential fields cannot be empty.\nPlease try again.",
					func() {
						tui.pages.SwitchToPage("page3")
					},
				)
				return
			}
			// Show a quick loading modal while validating credentials by attempting to fetch list of buckets/containers
			tui.showLoadingModal("Validating credentials...")
			go func() {
				err := tui.checkCredentials()

				tui.app.QueueUpdateDraw(func() {
					tui.pages.SwitchToPage("page3")
					tui.pages.RemovePage("loading")

					if err != nil {
						tui.showErrorModal(fmt.Sprintf("[red::b]ERROR:[-::-] %s", err.Error()),
							func() {
								tui.pages.SwitchToPage("page3")
							},
						)
						return
					}
					if tui.config.storageProtocol == "azstorage" {
						tui.pages.RemovePage("page4")
						tui.pages.SwitchToPage("page5")
					} else {
						page4 := tui.buildBucketSelectionPage()
						tui.pages.AddAndSwitchToPage("page4", page4, true)
					}
				})
			}()
		}).
		AddButton(tui.theme.navigationBackLabel, func() {
			if tui.config.storageProvider == "Microsoft" {
				tui.pages.RemovePage("page2")
				tui.pages.SwitchToPage("page1")
			} else {
				page2 := tui.buildEndpointURLPage()
				tui.pages.AddAndSwitchToPage("page2", page2, true)
			}
		}).
		AddButton(tui.theme.navigationPreviewLabel, func() {
			previewPage := tui.buildPreviewPage("page3")
			tui.pages.AddAndSwitchToPage("previewPage", previewPage, true)
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetButtonTextColor(tui.theme.navigationButtonTextColor)

	// Combine all credential widgets into a single form
	credentialsWidget := tview.NewForm().
		AddFormItem(accessKeyFieldWidget).
		AddFormItem(secretKeyFieldWidget).
		SetFieldTextColor(tcell.ColorBlack).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor)

	// If Azure is selected, add the container name field
	if tui.config.storageProvider == "Microsoft" {
		credentialsWidget.AddFormItem(containerNameFieldWidget)
	}

	credentialsWidget.AddFormItem(passphraseFieldWidget)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(credentialsWidget, credentialsWidget.GetFormItemCount()*2+1, 0, false).
		AddItem(navigationButtonsWidget, tui.theme.navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 4: Bucket Name Selection ---
//
// Function to build the bucket selection page. Allows users to select a bucket from a dropdown list
// of retrieved buckets based on provided s3 credentials. For s3 storage users only. Azure storage users will skip this page.
func (tui *appContext) buildBucketSelectionPage() tview.Primitive {
	instructionsText := fmt.Sprintf(
		"[%s::b] Select Your Bucket Name[-::-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n"+
			"[white::b] Select the name of your storage bucket from the dropdown below.[-::-]\n\n"+
			"[grey::i] The list of available buckets is retrieved from your cloud storage provider\n "+
			"based on the credentials provided in the previous step.[-::-]",
		colorGreen, colorYellow)

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Dropdown widget for selecting bucket name
	bucketSelectionWidget := tview.NewDropDown().
		SetLabel(" 🪣 Bucket Name: ").
		SetOptions(tui.config.bucketList, func(name string, index int) {
			tui.config.bucketName = name
		}).
		SetCurrentOption(0).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack).
		SetFieldWidth(25)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(tui.theme.navigationHomeLabel, func() {
			tui.pages.SwitchToPage("home")
		}).
		AddButton(tui.theme.navigationNextLabel, func() {
			tui.pages.SwitchToPage("page5")
		}).
		AddButton(tui.theme.navigationBackLabel, func() {
			tui.pages.SwitchToPage("page3")
		}).
		AddButton(tui.theme.navigationPreviewLabel, func() {
			previewPage := tui.buildPreviewPage("page4")
			tui.pages.AddAndSwitchToPage("previewPage", previewPage, true)
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetButtonTextColor(tui.theme.navigationButtonTextColor)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(bucketSelectionWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, tui.theme.navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 5: Caching Settings ---
//
// Function to build the caching page that allows users to configure caching settings.
// Includes options for enabling/disabling caching, specifying cache location, size, and retention settings.
func (tui *appContext) buildCachingPage() tview.Primitive {
	// Main layout container. Must be instantiated first to allow nested items.
	layout := tview.NewFlex().SetDirection(tview.FlexRow)

	instructionsText := fmt.Sprintf(
		"[%s::b] Configure Caching Settings[-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"[white::b] CloudFuse can cache data locally. You control the location, size, and duration.[-::-]\n\n"+
			"[%s::b]  -[-::-] [%s::b]Enable[-::-] caching if you frequently re-read data and have ample disk space.\n"+
			"[%s::b]  -[-::-] [red::b]Disable[-::-] caching if you prefer faster initial access or have limited disk space.\n\n",
		colorGreen, colorYellow, colorYellow, colorGreen, colorYellow)

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Dropdown widget for enabling/disabling caching
	cacheLocationFieldWidget := tview.NewInputField().
		SetLabel("📁 Cache Location: ").
		SetText(tui.config.cacheLocation).
		SetFieldWidth(40).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack).
		SetChangedFunc(func(location string) {
			tui.config.cacheLocation = location
		})

		// Input field widget for cache size percentage
	cacheSizeFieldWidget := tview.NewInputField().
		SetLabel("📊 Cache Size (%): ").
		SetText(tui.config.cacheSize). // Default to 80%
		SetFieldWidth(4).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack).
		SetChangedFunc(func(size string) {
			if size, err := strconv.Atoi(size); err != nil || size < 1 || size > 100 {
				tui.showErrorModal(
					"[red::b]ERROR:[-::-] Cache size must be between 1 and 100.\nPlease try again.",
					func() {
						tui.pages.SwitchToPage("page5")
					},
				)
				return
			}
			tui.config.cacheSize = size
		})

	// Input field widget for cache retention duration
	cacheRetentionDurationFieldWidget := tview.NewInputField().
		SetLabel("⌛ Cache Retention Duration: ").
		SetText(fmt.Sprintf("%d", tui.config.cacheRetentionDuration)).
		SetFieldWidth(5).
		SetChangedFunc(func(duration string) {
			if val, err := strconv.Atoi(duration); err == nil {
				tui.config.cacheRetentionDuration = val
			} else {
				// TODO: Handle invalid input
				tui.config.cacheRetentionDuration = 0
			}
		}).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Dropdown widget for cache retention unit
	cacheRetentionUnitDropdownWidget := tview.NewDropDown().
		SetOptions([]string{"Seconds", "Minutes", "Hours", "Days"}, func(option string, index int) {
			tui.config.cacheRetentionUnit = option
			// Convert cache retention duration to seconds
			switch tui.config.cacheRetentionUnit {
			case "Seconds":
				tui.config.cacheRetentionDurationSec = tui.config.cacheRetentionDuration
			case "Minutes":
				minutes := tui.config.cacheRetentionDuration
				tui.config.cacheRetentionDurationSec = minutes * 60
			case "Hours":
				hours := tui.config.cacheRetentionDuration
				tui.config.cacheRetentionDurationSec = hours * 3600
			case "Days":
				days := tui.config.cacheRetentionDuration
				tui.config.cacheRetentionDurationSec = days * 86400
			}
		}).
		SetCurrentOption(3). // Default to Days
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Dropdown widget for enabling/disabling cache cleanup on restart
	// If enabled --> allow-non-empty-temp: false
	// if disabled --> allow-non-empty-temp: true
	clearCacheOnStartDropdownWidget := tview.NewDropDown().
		SetLabel("🧹 Clear Cache On Start: ").
		SetOptions([]string{" Enabled ", " Disabled "}, func(option string, index int) {
			if option == " Enabled " {
				tui.config.clearCacheOnStart = true
			} else {
				tui.config.clearCacheOnStart = false
			}
		}).SetCurrentOption(0).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(colorBlack)

	// Horizontal container to place retention duration and unit side by side
	cacheRetentionRow := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(cacheRetentionDurationFieldWidget, 35, 0, false).
		AddItem(cacheRetentionUnitDropdownWidget, 7, 0, false)

	// Group cache field widgets in a container
	cacheFields := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(cacheLocationFieldWidget, 2, 0, false).
		AddItem(cacheSizeFieldWidget, 2, 0, false).
		AddItem(cacheRetentionRow, 2, 0, false).
		AddItem(clearCacheOnStartDropdownWidget, 2, 0, false)

	// Tracks whether or not cache fields are currently shown
	showCacheFields := true

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm()
	navigationButtonsWidget.
		AddButton(tui.theme.navigationHomeLabel, func() {
			tui.pages.SwitchToPage("home")
		}).
		AddButton(tui.theme.navigationFinishLabel, func() {
			// Check if caching is enabled and validate cache settings
			if tui.config.enableCaching {
				// Validate the cache location
				if err := tui.validateCachePath(); err != nil {
					tui.showErrorModal(
						"[red::b]ERROR:[-::-] Invalid cache location:\n"+err.Error(),
						func() {
							tui.pages.SwitchToPage("page5")
						},
					)
					return
				}

				// Check available cache size
				if err := tui.getAvailableCacheSize(); err != nil {
					tui.showErrorModal(
						"[red::b]ERROR:[-::-] Failed to fetch available cache size:\n"+err.Error(),
						func() {
							tui.pages.SwitchToPage("page5")
						},
					)
					return
				}

				cacheSizeText := fmt.Sprintf(
					"Available Disk Space @ Cache Location: [darkred::b]%d GB[-::-]\n",
					tui.config.availableCacheSizeGB,
				) +
					fmt.Sprintf(
						"Cache Size Currently Set to: [darkred::b]%.0f GB (%s%%)[-::-]\n\n",
						float64(tui.config.currentCacheSizeGB),
						tui.config.cacheSize,
					) +
					"Would you like to proceed with this cache size?\n\n" +
					"If not, hit [darkred::b]Return[-::-] to adjust cache size accordingly. Otherwise, hit [darkred::b]Finish[-::-] to complete the configuration."

				tui.showCacheConfirmationModal(cacheSizeText,
					// Callback function if the user selects Finish
					func() {
						if err := tui.createYAMLConfig(); err != nil {
							tui.showErrorModal(
								"[red::b]ERROR:[-::-] Failed to create YAML config:\n"+err.Error(),
								func() {
									tui.pages.SwitchToPage("page5")
								},
							)
							return
						}
						tui.showExitModal(func() {
							tui.app.Stop()
						})
					},
					// Callback function if the user selects Return
					func() {
						tui.pages.SwitchToPage("page5")
					})

			} else {
				// If caching is disabled, just finish the configuration
				if err := tui.createYAMLConfig(); err != nil {
					tui.showErrorModal("[red::b]ERROR:[-::-] Failed to create YAML config:\n"+err.Error(), func() {
						tui.pages.SwitchToPage("page5")
					})
					return
				}
				tui.showExitModal(func() {
					tui.app.Stop()
				})
			}
		}).
		AddButton(tui.theme.navigationBackLabel, func() {
			if tui.config.storageProtocol == "azstorage" {
				tui.pages.SwitchToPage("page3")
			} else {
				tui.pages.SwitchToPage("page4")
			}
		}).
		AddButton(tui.theme.navigationPreviewLabel, func() {
			previewPage := tui.buildPreviewPage("page5")
			tui.pages.AddAndSwitchToPage("previewPage", previewPage, true)
		}).
		AddButton(tui.theme.navigationQuitLabel, func() {
			tui.app.Stop()
		}).
		SetButtonBackgroundColor(tui.theme.navigationButtonColor).
		SetButtonTextColor(colorBlack)

		// Widget to enable/disable caching
	enableCachingDropdownWidget := tview.NewDropDown()
	enableCachingDropdownWidget.
		SetLabel("💾 Caching: ").
		SetOptions([]string{" Enabled ", " Disabled "}, func(option string, index int) {
			if option == " Enabled " {
				tui.config.cacheMode = "file_cache"
				tui.config.enableCaching = true
				if !showCacheFields {
					layout.RemoveItem(navigationButtonsWidget)
					layout.RemoveItem(cacheFields)
					layout.AddItem(cacheFields, 8, 0, false)
					layout.AddItem(
						navigationButtonsWidget,
						tui.theme.navigationWidgetHeight,
						0,
						false,
					)
					showCacheFields = true
				}
			} else {
				tui.config.cacheMode = "block_cache"
				tui.config.enableCaching = false
				if showCacheFields {
					layout.RemoveItem(cacheFields)
					showCacheFields = false
				}
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(tui.theme.widgetLabelColor).
		SetFieldBackgroundColor(tui.theme.widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

		// Assemble page layout
	layout.AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false)
	layout.AddItem(enableCachingDropdownWidget, 2, 0, false)

	if showCacheFields {
		layout.AddItem(cacheFields, 8, 0, false)
	}

	layout.AddItem(navigationButtonsWidget, tui.theme.navigationWidgetHeight, 0, false)
	layout.AddItem(nil, 1, 0, false)
	layout.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Summary Page ---
//
// Function to build the summary page that displays the configuration summary.
// This function creates a text view with the summary information and a return button.
// The preview page parameter allows switching back to the previous page when the user clicks "Return".
func (tui *appContext) buildPreviewPage(previewPage string) tview.Primitive {
	summaryText := fmt.Sprintf(
		"[%s::b] CloudFuse Summary Configuration:[-]\n"+
			"[%s]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n[-]"+
			" Storage Provider: [%s::b]%s[-]\n"+
			"     Endpoint URL: [%s::b]%s[-]\n"+
			"      Bucket Name: [%s::b]%s[-]\n"+
			"       Cache Mode: [%s::b]%s[-]\n"+
			"   Cache Location: [%s::b]%s[-]\n"+
			"       Cache Size: [%s::b]%s%% (%d GB)[-]\n",
		colorGreen, colorYellow,
		colorYellow, tui.config.storageProvider,
		colorYellow, tui.config.endpointURL,
		colorYellow, tui.config.bucketName,
		colorYellow, tui.config.cacheMode,
		colorYellow, tui.config.cacheLocation,
		colorYellow, tui.config.cacheSize, tui.config.currentCacheSizeGB,
	)

	// Display cache retention duration in seconds and specified unit
	if tui.config.cacheRetentionUnit == "Seconds" {
		summaryText += fmt.Sprintf(
			"  Cache Retention: [%s::b]%d Seconds[-]\n\n",
			colorYellow, tui.config.cacheRetentionDurationSec,
		)
	} else {
		summaryText += fmt.Sprintf("  Cache Retention: [%s::b]%d sec (%d %s)[-]\n\n",
			colorYellow, tui.config.cacheRetentionDurationSec, tui.config.cacheRetentionDuration, tui.config.cacheRetentionUnit)
	}

	// Set a dynamic width and height for the summary widget
	summaryWidgetHeight := getTextHeight(summaryText)
	summaryWidgetWidth := getTextWidth(summaryText) / 3

	summaryWidget := tview.NewTextView().
		SetWrap(true).
		SetDynamicColors(true).
		SetText(summaryText).
		SetScrollable(true)

	returnButton := tview.NewButton("[black]Return[-]").
		SetSelectedFunc(func() {
			tui.pages.SwitchToPage(previewPage)
		})
	returnButton.SetBackgroundColor(colorGreen)
	returnButton.SetBorder(true)
	returnButton.SetBorderColor(colorYellow)
	returnButton.SetBackgroundColorActivated(colorGreen)

	buttons := tview.NewFlex().
		SetDirection(tview.FlexColumn).
		AddItem(nil, 0, 1, false). // Left button spacer
		AddItem(returnButton, 20, 0, true).
		AddItem(nil, 0, 1, false) // Right button spacer

	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(summaryWidget, summaryWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(buttons, 3, 0, true)

	leftAlignedModal := tview.NewFlex().
		AddItem(modal, summaryWidgetWidth, 0, true)

	leftAlignedModal.SetBorder(true).SetBorderColor(colorGreen).SetBorderPadding(1, 1, 1, 1)

	return leftAlignedModal
}

// Function to show a modal dialog with a message and an "OK" button.
// This function is used to display error messages or confirmations.
// May specify a callback function to execute when the modal is closed.
func (tui *appContext) showErrorModal(message string, onClose func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			tui.pages.RemovePage("modal")
			onClose()
		}).
		SetBackgroundColor(colorGreen).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(colorYellow)
	modal.SetButtonBackgroundColor(colorYellow)
	modal.SetButtonTextColor(tcell.ColorBlack)
	tui.pages.AddPage("modal", modal, false, true)
}

// Function to show a loading modal dialog with a message.
func (tui *appContext) showLoadingModal(loadingMessage string) {
	modal := tview.NewModal().
		SetText(loadingMessage).
		SetBackgroundColor(colorGreen).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(colorYellow)
	modal.SetButtonBackgroundColor(colorYellow)
	modal.SetButtonTextColor(tcell.ColorBlack)
	tui.pages.AddPage("loading", modal, true, true)
}

// Function to show a confirmation modal dialog with "Finish" and "Return" buttons.
// Used to confirm cache size before proceeding. Must specify two callback functions for the "Finish" and "Return" actions.
func (tui *appContext) showCacheConfirmationModal(
	message string,
	onFinish func(),
	onReturn func(),
) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Finish", "Return"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			tui.pages.RemovePage("modal")
			if buttonLabel == "Finish" {
				onFinish()
			} else {
				onReturn()
			}
		}).
		SetBackgroundColor(colorGreen).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(colorYellow)
	modal.SetButtonBackgroundColor(colorYellow)
	modal.SetButtonTextColor(tcell.ColorBlack)
	tui.pages.AddPage("modal", modal, true, true)
}

// Function to show final exit modal when configuration is complete.
// Informs the user that the configuration is complete and they can exit.
// This function is called when the user clicks "Finish" on the caching page.
func (tui *appContext) showExitModal(onConfirm func()) {

	processingEmojis := []string{"🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚", "✅"}

	modal := tview.NewModal().
		AddButtons([]string{"Exit"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			tui.pages.RemovePage("modal")
			if buttonLabel == "Exit" {
				onConfirm()
			}
		}).
		SetBackgroundColor(colorGreen).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(colorYellow)
	modal.SetButtonBackgroundColor(colorYellow)
	modal.SetButtonTextColor(tcell.ColorBlack)

	tui.pages.AddPage("modal", modal, true, true)

	// Simulate processing with emoji animation
	go func() {
		// Show initial message with emoji animation
		for i := 0; i < len(processingEmojis); i++ {
			currentEmoji := processingEmojis[i]
			time.Sleep(100 * time.Millisecond)
			tui.app.QueueUpdateDraw(func() {
				modal.SetText(
					fmt.Sprintf(
						"[black::b]Creating configuration file...[-::-]\n\n%s",
						currentEmoji,
					),
				)
			})
		}

		// After animation, show final message
		tui.app.QueueUpdateDraw(func() {
			modal.SetText(fmt.Sprintf("[black::b]Configuration Complete![-::-]\n\n%s\n\n"+
				"Your CloudFuse configuration file has been created at:\n\n[blue:white:b] %s [-:-:-]\n\n"+
				"You can now exit the application.\n\n"+
				"[black::i]Thank you for using CloudFuse Config![-::-]", processingEmojis[len(processingEmojis)-1], tui.config.configFilePath))
		})
	}()
}

// Helper function to center lines of text within a specified width.
// It is used to format text views and other UI elements in the TUI.
func centerText(text string, width int) string {
	var centeredLines []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		visibleLen := tview.TaggedStringWidth(line) // handle color tags
		if visibleLen >= width {
			centeredLines = append(centeredLines, line)
		} else {
			padding := (width - visibleLen) / 2
			centeredLines = append(centeredLines, strings.Repeat(" ", padding)+line)
		}
	}
	return strings.Join(centeredLines, "\n")
}

// Helper function to get the length of the longest line in a string.
// It is used to determine the width of text views and other UI elements.
func getTextWidth(s string) int {
	if s == "" {
		return 0
	}
	lines := strings.Split(s, "\n")
	longest := 0
	for _, line := range lines {
		if len(line) > longest {
			longest = len(line)
		}
	}
	return longest
}

// Helper function to count the number of lines in a string.
// It is used to determine the height of text views and other UI elements.
func getTextHeight(s string) int {
	if s == "" {
		return 0
	}
	return len(strings.Split(s, "\n"))
}

// Helper function to get a fallback cache path if the home directory cannot be determined.
func getFallbackCachePath() string {
	user := os.Getenv("USER")
	if user == "" {
		uid := os.Getuid()
		user = fmt.Sprintf("uid_%d", uid)
	}
	return filepath.Join(os.TempDir(), "cloudfuse", user)
}

// Helper function to get the default cache path.
// It retrieves the user's home directory and constructs a default cache path:
//
//	`~/.cloudfuse/file_cache`. If it fails to retrieve the home directory or create the path, it returns a fallback path.
func getDefaultCachePath() string {
	// TODO: Add logic to return OS-specific cache paths
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf(
			"Failed to get home directory: %v\nUsing fallback path for cache directory.\n",
			err,
		)
		return getFallbackCachePath()
	}
	cachePath := filepath.Join(home, ".cloudfuse", "file_cache")
	// If the directory doesn't exist, create it
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if err := os.MkdirAll(cachePath, 0700); err != nil {
			fmt.Printf(
				"Failed to create cache directory: %v\nUsing fallback path for cache directory.\n",
				err,
			)
			return getFallbackCachePath()
		}
	}
	// Return the full path to the cache directory
	return cachePath
}

// Helper function to validate the entered cache path.
func (tui *appContext) validateCachePath() error {
	// Validate that the path is not empty
	if strings.TrimSpace(tui.config.cacheLocation) == "" {
		return fmt.Errorf("Cache location cannot be empty.")
	}
	// Make sure no invalid path characters are used
	if strings.ContainsAny(tui.config.cacheLocation, `<>:"|?*#%^&;'"`+"`"+`{}[]`) {
		return fmt.Errorf("Cache location contains invalid characters.")
	}
	// Validate that the cache path exists
	if tui.config.cacheLocation != getDefaultCachePath() && tui.config.cacheMode == "file_cache" {
		if _, err := os.Stat(tui.config.cacheLocation); os.IsNotExist(err) {
			return fmt.Errorf("'%s': No such file or directory.", tui.config.cacheLocation)
		}
	}
	return nil
}

// Helper function to get the available disk space at the cache location and calculates
// the cache size in GB based on the user-defined cache size percentage.
func (tui *appContext) getAvailableCacheSize() error {
	availableBlocks, _, err := common.GetAvailFree(tui.config.cacheLocation)
	if err != nil {
		// If we fail to get the available cache size, we default to 80% of the available disk space
		tui.config.cacheSize = "80"
		returnMsg := fmt.Errorf(
			"Failed to get available cache size at '%s': %v\n\n"+
				"Defaulting cache size to 80%% of available disk space.\n\n"+
				"Please manually verify you have enough disk space available for caching.",
			tui.config.cacheLocation, err)
		return returnMsg
	}

	const blockSize = 4096
	availableCacheSizeBytes := availableBlocks * blockSize // Convert blocks to bytes
	tui.config.availableCacheSizeGB = int(
		availableCacheSizeBytes / (1024 * 1024 * 1024),
	) // Convert to GB
	cacheSizeInt, _ := strconv.Atoi(tui.config.cacheSize)
	tui.config.currentCacheSizeGB = int(tui.config.availableCacheSizeGB) * cacheSizeInt / 100

	return nil
}

// Helper function to normalize and validate the user-defined endpoint URL.
func (tui *appContext) validateEndpointURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)

	// Check if the URL is empty
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("Endpoint URL cannot be empty.\nPlease try again.")
	}

	// Normalize the URL by adding "https://" if it doesn't start with "http://" or "https://"
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		tui.config.endpointURL = "https://" + rawURL
		return fmt.Errorf(
			"Endpoint URL should start with 'http://' or 'https://'.\n" +
				"Appending 'https://' to the URL...\n\nPlease verify the URL and try again.",
		)
	}

	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return fmt.Errorf(
			"Invalid URL format.\n%s\nPlease try again.", err.Error())
	}

	return nil
}

// Function to check the credentials entered by the user.
// Attempts to connect to the storage backend and fetch the bucket list.
// If successful, populates the `bucketList` variable with the list of available buckets (for s3 providers only).
// Called when the user clicks "Next" on the credentials page.
func (tui *appContext) checkCredentials() error {
	// Create a temporary configOptions struct with only the storage component
	tmpConfig := configOptions{
		Components: []string{tui.config.storageProtocol},
	}

	if tui.config.storageProtocol == "azstorage" {
		tmpConfig.AzStorage = azstorage.AzStorageOptions{
			AccountType: "block",
			AccountName: tui.config.accountName,
			AccountKey:  tui.config.accountKey,
			AuthMode:    "key",
			Container:   tui.config.containerName,
		}
	} else {
		tmpConfig.S3Storage = s3StorageConfig{
			BucketName:      tui.config.bucketName,
			KeyID:           tui.config.accessKey,
			SecretKey:       tui.config.secretKey,
			Endpoint:        tui.config.endpointURL,
			EnableDirMarker: true,
		}
	}

	// Marshal the temporary struct to YAML format
	tmpConfigData, _ := yaml.Marshal(&tmpConfig)

	// Write the temporary config data into the global options struct instead of a temporary file.
	// This avoids the need to create and delete a temporary file on disk.
	if err := config.ReadFromConfigBuffer(tmpConfigData); err != nil {
		return fmt.Errorf("Failed to read config from buffer: %v", err)
	}

	if err := config.Unmarshal(&options); err != nil {
		return fmt.Errorf("Failed to unmarshal config: %v", err)
	}

	// Try to fetch bucket list
	var err error
	if slices.Contains(options.Components, "azstorage") {
		tui.config.bucketList, err = getContainerListAzure()

	} else if slices.Contains(options.Components, "s3storage") {
		tui.config.bucketList, err = getBucketListS3()

	} else {
		err = fmt.Errorf("Unsupported storage backend")
	}

	if err != nil {
		return fmt.Errorf("Failed to validate credentials: %v", err)
	}

	return nil
}

// Function to create the YAML configuration file based on user inputs once all forms are completed.
// Called when the user clicks "Finish" on the caching page.
func (tui *appContext) createYAMLConfig() error {
	config := configOptions{
		Components: []string{
			"libfuse",
			tui.config.cacheMode,
			"attr_cache",
			tui.config.storageProtocol,
		},

		Libfuse: libfuse.LibfuseOptions{
			NetworkShare: true,
		},

		AttrCache: attr_cache.AttrCacheOptions{
			Timeout: uint32(7200),
		},
	}

	if tui.config.cacheMode == "file_cache" {
		config.FileCache = file_cache.FileCacheOptions{
			TmpPath:       tui.config.cacheLocation,
			Timeout:       uint32(tui.config.cacheRetentionDurationSec),
			AllowNonEmpty: !tui.config.clearCacheOnStart,
			SyncToFlush:   true,
		}
		// If cache size is not set to 80%, convert currentCacheSizeGB to MB and set file_cache.max-size-mb to it
		if tui.config.cacheSize != "80" {
			config.FileCache.MaxSizeMB = float64(
				tui.config.currentCacheSizeGB * 1024,
			) // Convert GB to MB
		}
	}

	if tui.config.storageProtocol == "s3storage" {
		config.S3Storage = s3StorageConfig{
			BucketName:      tui.config.bucketName,
			KeyID:           tui.config.accessKey,
			SecretKey:       tui.config.secretKey,
			Endpoint:        tui.config.endpointURL,
			EnableDirMarker: true,
		}
	} else {
		config.AzStorage = azstorage.AzStorageOptions{
			AccountType: "block",
			AccountName: tui.config.accountName,
			AccountKey:  tui.config.accountKey,
			AuthMode:    "key",
			Container:   tui.config.containerName,
		}
	}

	// Marshal the struct to YAML format
	configData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf(
			"Failed to marshal configuration data to YAML: %v", err)
	}

	// Encrypt the YAML config data using the user-provided passphrase
	encryptedPassphrase := memguard.NewEnclave([]byte(tui.config.configEncryptionPassphrase))
	cipherText, err := common.EncryptData(configData, encryptedPassphrase)
	if err != nil {
		return fmt.Errorf("Failed to encrypt configuration data: %v", err)
	}

	// Write the encrypted YAML config data to a file
	if err := os.WriteFile("config.aes", cipherText, 0600); err != nil {
		return fmt.Errorf("Failed to create encrypted config.aes file: %v", err)
	}

	// Update configFilePath member to point to the created config file
	currDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Failed to get current working directory: %v", err)
	}

	tui.config.configFilePath = filepath.Join(currDir, "config.aes")

	return nil
}
