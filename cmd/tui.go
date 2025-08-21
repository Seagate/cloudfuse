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
	"github.com/Seagate/cloudfuse/component/attr_cache"
	"github.com/Seagate/cloudfuse/component/azstorage"
	"github.com/Seagate/cloudfuse/component/file_cache"
	"github.com/Seagate/cloudfuse/component/libfuse"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)

// Constants and global variables used throughout the TUI application.
// These include default values, colors, widget configurations, and storage settings.
var (
	tuiVersion                string       = common.CloudfuseVersion // Mirrors the current version of cloudfuse.
	configFilePath            string                                 // Sets file_cache.path
	accountName               string                                 // Sets azstorage.account-name
	accountKey                string                                 // Sets azstorage.account-key
	accessKey                 string                                 // Sets s3storage.key-id
	secretKey                 string                                 // Sets s3storage.secret-key
	containerName             string                                 // Sets azstorage.container-name
	bucketName                string                                 // Sets s3storage.bucket-name
	endpointURL               string                                 // Sets s3storage.endpoint
	bucketList                = []string{}                           // Holds list of available buckets retrieved from cloud provider (for s3 only).
	storageProtocol           string                                 // Sets 's3storage' or 'azstorage' based on selected provider
	storageProvider           string                                 // Options: 'LyveCloud', 'Microsoft', 'AWS', or 'Other (s3)'. Used to set certain UI elements.
	cacheMode                 string                                 // Sets 'components' to include 'file_cache' or 'block_cache'
	enableCaching             bool         = true                    // If true, sets cacheMode to file_cache. If false, block_cache
	cacheLocation             string       = getDefaultCachePath()   // Sets file_cache.path @ startup to default: $HOME/.cloudfuse/cache
	cacheSize                 string       = "80"                    // User-defined cache size as %
	availableCacheSizeGB      int                                    // Total available cache size in GB @ the cache location
	currentCacheSizeGB        int                                    // Current cache size in GB based on 'cacheSize' percentage
	clearCacheOnStart         bool         = false                   // If false, sets 'allow-non-empty-temp' to true
	cacheRetentionDuration    int          = 2                       // User-defined cache retention duration. Default is '2'
	cacheRetentionUnit        string                                 // User-defined cache retention unit (sec, min, hours, days). Default is 'days'
	cacheRetentionDurationSec int                                    // Sets 'file_cache.timeout-sec' from 'cacheRetentionDuration'

	// Global variables for UI elements
	tuiAlignment                           = tview.AlignLeft
	yellowColor                tcell.Color = tcell.GetColor("#FFD700")
	greenColor                 tcell.Color = tcell.GetColor("#6EBE49")
	widgetLabelColor                       = yellowColor
	widgetFieldBackgroundColor             = yellowColor
	navigationButtonColor                  = greenColor
	navigationButtonTextColor              = tcell.ColorBlack
	navigationButtonAlignment              = tview.AlignLeft
	navigationStartLabel       string      = "[black]🚀 Start[-]"
	navigationHomeLabel        string      = "[black]🏠 Home[-]"
	navigationNextLabel        string      = "[black]🡲  Next[-]"
	navigationBackLabel        string      = "[black]🡰  Back[-]"
	navigationPreviewLabel     string      = "[black]📄 Preview[-]"
	navigationQuitLabel        string      = "[black]❌ Quit[-]"
	navigationFinishLabel      string      = "[black]✅ Finish[-]"
	navigationWidgetHeight     int         = 3
)

type configuration struct {
	Components []string                    `yaml:"components,omitempty"`
	Libfuse    libfuse.LibfuseOptions      `yaml:"libfuse,omitempty"`
	FileCache  file_cache.FileCacheOptions `yaml:"file_cache,omitempty"`
	AttrCache  attr_cache.AttrCacheOptions `yaml:"attr_cache,omitempty"`
	S3Storage  s3StorageConfig             `yaml:"s3storage,omitempty"`
	AzStorage  azstorage.AzStorageOptions  `yaml:"azstorage,omitempty"`
}

type s3StorageConfig struct {
	BucketName      string `yaml:"bucket-name,omitempty"`
	KeyID           string `yaml:"key-id"`
	SecretKey       string `yaml:"secret-key"`
	Endpoint        string `yaml:"endpoint"`
	EnableDirMarker bool   `yaml:"enable-dir-marker"`
}

// Main function to run the TUI application.
// Initializes the tview application, builds the TUI application, and runs it.
func runTUI() error {
	app := tview.NewApplication()
	app.EnableMouse(true)
	app.EnablePaste(true)

	buildTUI(app)

	// Run the application
	if err := app.Run(); err != nil {
		panic(err)
	}

	return nil
}

// Function to build the TUI application. Initializes the pages and adds them to the page stack.
func buildTUI(app *tview.Application) {
	pages := tview.NewPages()

	// Initialize the pages
	homePage := buildHomePage(app, pages)         // --- Home Page ---
	page1 := buildStorageProviderPage(app, pages) // --- Page 1: Storage Provider Selection ---
	page2 := buildEndpointURLPage(app, pages)     // --- Page 2: Endpoint URL Entry ---
	page3 := buildCredentialsPage(app, pages)     // --- Page 3: Credentials Entry ---
	page4 := buildBucketSelectionPage(app, pages) // --- Page 4: Bucket Selection ---
	page5 := buildCachingPage(app, pages)         // --- Page 5: Caching Settings ---

	// Add pages to the page stack
	pages.AddPage("home", homePage, true, true)
	pages.AddPage("page1", page1, true, false)
	pages.AddPage("page2", page2, true, false)
	pages.AddPage("page3", page3, true, false)
	pages.AddPage("page4", page4, true, false)
	pages.AddPage("page5", page5, true, false)

	app.SetRoot(pages, true)
}

//	--- Page 0: Home Page ---
//
// Function to build the home page of the TUI application. Displays a
// welcome banner, instructions, and buttons to start or quit the application.
func buildHomePage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	bannerText := "[#6EBE49::b]░█▀▀░█░░░█▀█░█░█░█▀▄░█▀▀░█░█░█▀▀░█▀▀\n" +
		"░█░░░█░░░█░█░█░█░█░█░█▀▀░█░█░▀▀█░█▀▀\n" +
		"░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀░░░▀▀▀░▀▀▀░▀▀▀[-]\n\n" +
		"[white::b]Welcome to the CloudFuse Configuration Tool\n" +
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n" +
		"[#6EBE49::b]Cloud storage configuration made easy via terminal.[-]\n\n" +
		"[::b]Press [#FFD700]Start[-] to begin or [red]Quit[-] to exit.\n"

	// Banner text widget
	bannerTextWidget := tview.NewTextView().
		SetText(centerText(bannerText, 75)).
		SetTextAlign(tuiAlignment).
		SetDynamicColors(true).
		SetWrap(true)

	instructionsText := "[#FFD700::b]Instructions:[::-]\n" +
		"[#6EBE49::b]•[-::-] [::]Use your mouse or arrow keys to navigate.[-::-]\n" +
		"[#6EBE49::b]•[-::-] [::]Press Enter or left-click to select items.[-::-]\n" +
		"[#6EBE49::b]•[-::-] [::]For the best experience, expand terminal window to full size.[-::-]\n"

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetDynamicColors(true).
		SetTextAlign(tuiAlignment).
		SetWrap(true)

	// Start/Quit buttons widget
	startQuitButtonsWidget := tview.NewForm().
		AddButton(navigationStartLabel, func() {
			pages.SwitchToPage("page1")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(navigationButtonColor).
		SetButtonTextColor(navigationButtonTextColor).
		SetButtonsAlign(navigationButtonAlignment)

	aboutText := "[#FFD700::b]ABOUT[-::-]\n" +
		"[white]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n" +
		"[grey::i]CloudFuse TUI Configuration Tool\n" +
		"Seagate Technology, LLC\n" +
		"cloudfuse@seagate.com\n" +
		fmt.Sprintf("Version: %s", tuiVersion)

	// About text widget
	aboutTextWidget := tview.NewTextView().
		SetText(centerText(aboutText, 75)).
		SetDynamicColors(true).
		SetTextAlign(tuiAlignment).
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

	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 1: Storage Provider Selection ---
//
// Function to build the storage provider selection page. Allows users to select their cloud storage provider
// from a dropdown list. The options are: LyveCloud, Microsoft, AWS, and Other S3.
func buildStorageProviderPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	instructionsText := "[#6EBE49::b] Select Your Cloud Storage Provider[-::-]\n" +
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n" +
		"[white::b] Choose your cloud storage provider from the dropdown below.[-::-]\n" +
		"[grey::i] If your provider is not listed, choose [darkmagenta::b]Other (s3)[-::-][grey::i]. You’ll be\n" +
		" prompted to enter the endpoint URL and region manually.[-::-]\n"

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetTextAlign(tuiAlignment).
		SetDynamicColors(true).
		SetWrap(true)

	// Dropdown widget for selecting storage provider
	storageProviderDropdownWidget := tview.NewDropDown().
		SetLabel("📦 Storage Provider: ").
		SetOptions([]string{" LyveCloud ⬇️", " Microsoft ", " AWS ", " Other (s3) "}, func(option string, index int) {
			storageProvider = option
			switch option {
			case " LyveCloud ⬇️":
				storageProtocol = "s3storage"
				storageProvider = "LyveCloud"
			case " Microsoft ":
				storageProtocol = "azstorage"
				storageProvider = "Microsoft"
			case " AWS ":
				storageProtocol = "s3storage"
				storageProvider = "AWS"
			case " Other (s3) ":
				storageProtocol = "s3storage"
				storageProvider = "Other"
				endpointURL = ""
			default:
				storageProtocol = "s3storage"
				storageProvider = "LyveCloud"
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(14)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(navigationHomeLabel, func() {
			pages.SwitchToPage("home")
		}).
		AddButton(navigationNextLabel, func() {
			// If Microsoft is selected, switch to page 3 and skip endpoint entry, handled internally by Azure SDK.
			if storageProvider == "Microsoft" {
				page3 := buildCredentialsPage(app, pages)
				pages.AddPage("page3", page3, true, false)
				pages.SwitchToPage("page3")
			} else {
				page2 := buildEndpointURLPage(app, pages)
				pages.AddPage("page2", page2, true, false)
				pages.SwitchToPage("page2")
			}
		}).
		AddButton(navigationPreviewLabel, func() {
			previewPage := buildPreviewPage(app, pages, "page1")
			pages.AddPage("previewPage", previewPage, true, false)
			pages.SwitchToPage("previewPage")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(navigationButtonColor).
		SetButtonTextColor(navigationButtonTextColor).
		SetButtonsAlign(navigationButtonAlignment)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(storageProviderDropdownWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 2: Endpoint URL Entry Page ---
//
// Function to build the endpoint URL page. Allows users to enter the endpoint URL for their cloud storage provider.
// It validates the endpoint URL format and provides help text based on the selected provider.
func buildEndpointURLPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	var urlRegionHelpText string

	// Determine URL help text based on selected provider
	switch storageProvider {
	case "LyveCloud":
		urlRegionHelpText = "[::b]You selected LyveCloud as your storage provider.[::-]\n\n" +
			"For LyveCloud, the endpoint URL format is generally:\n" +
			"[darkmagenta::b]https://s3.<[darkcyan::b]region[darkmagenta::b]>.<[darkcyan::b]identifier[darkmagenta::b]>.lyve.seagate.com[-]\n\n" +
			"Example:\n[darkmagenta::b]https://s3.us-east-1.sv15.lyve.seagate.com[-]\n\n" +
			"[grey::i]Find more info in your LyveCloud portal.\nAvailable regions are listed below in the dropdown.[-::-]"
		urlRegionHelpText = centerText(urlRegionHelpText, 65)

	case "AWS":
		urlRegionHelpText = "[::b]You selected AWS as your storage provider.[::-]\n\n" +
			"The endpoint URL format is generally:\n" +
			"[darkmagenta::b]https://s3.<[darkcyan::b]region[darkmagenta::b]>.amazonaws.com[-]\n\n" +
			"Example:\n[darkmagenta::b]https://s3.us-east-1.amazonaws.com[-]\n\n" +
			"[grey::i]Refer to AWS documentation for valid formats and available regions.[-::-]"
		urlRegionHelpText = centerText(urlRegionHelpText, 65)

	case "Other":
		urlRegionHelpText = "[::b]You selected a custom s3 provider.[::-]\n\n" +
			"Enter the endpoint URL.\n" +
			"[grey::i]Refer to your provider’s documentation for valid formats.[-::-]"
		urlRegionHelpText = centerText(urlRegionHelpText, 65)
	}

	instructionsText := fmt.Sprintf("[#6EBE49::b] Enter Endpoint URL for %s[-]\n"+
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"+
		"[white]\n %s", storageProvider, urlRegionHelpText)

	instructionsTextWidget := tview.NewTextView().
		SetText(instructionsText).
		SetTextAlign(tuiAlignment).
		SetWrap(true).
		SetDynamicColors(true)

	endpointURLFieldWidget := tview.NewInputField().
		SetLabel("🔗 Endpoint URL: ").
		SetText(endpointURL).
		SetFieldWidth(50).
		SetChangedFunc(func(url string) {
			endpointURL = url
		}).
		SetPlaceholder("\t\t\t\t<ENTER URL HERE>").
		SetPlaceholderTextColor(tcell.ColorGray).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(navigationHomeLabel, func() {
			pages.SwitchToPage("home")
		}).
		AddButton(navigationNextLabel, func() {
			if err := validateEndpointURL(endpointURL); err != nil {
				showErrorModal(
					app,
					pages,
					fmt.Sprintf("[red::b]ERROR: %s[-::-]", err.Error()),
					func() {
						pages.RemovePage("page2")
						page2 := buildEndpointURLPage(app, pages)
						pages.AddPage("page2", page2, true, false)
						pages.SwitchToPage("page2")
					},
				)
				return
			}
			pages.RemovePage("page3")
			page3 := buildCredentialsPage(app, pages)
			pages.AddPage("page3", page3, true, false)
			pages.SwitchToPage("page3")
		}).
		AddButton(navigationBackLabel, func() {
			pages.SwitchToPage("page1")
		}).
		AddButton(navigationPreviewLabel, func() {
			previewPage := buildPreviewPage(app, pages, "page2")
			pages.AddPage("previewPage", previewPage, true, false)
			pages.SwitchToPage("previewPage")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(navigationButtonColor).
		SetLabelColor(widgetLabelColor).
		SetButtonTextColor(navigationButtonTextColor).
		SetButtonsAlign(navigationButtonAlignment)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(endpointURLFieldWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 3: Credentials Page ---
//
// Function to build the credentials page. Allows users to enter their cloud storage credentials.
// If the storage protocol is "s3", it provides input fields for access key, secret key.
// If the storage protocol is "azure", it provides input fields for account name, account key, and container name.
func buildCredentialsPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	layout := tview.NewFlex()
	layout.Clear()

	// Determine labels for input fields based on storage protocol.
	accessLabel := ""
	secretLabel := ""
	if storageProtocol == "azstorage" {
		accessLabel = "🔑 Account Name: "
		secretLabel = "🔑 Account Key: "
	} else {
		accessLabel = "🔑 Access Key: "
		secretLabel = "🔑 Secret Key: "
	}

	instructionsText := fmt.Sprintf("[#6EBE49::b] Enter Your Cloud Storage Credentials[-]\n"+
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[::-]\n\n"+
		"[#FFD700::b] -[-::-] [#FFD700::b]%s[-::-] This is your unique identifier for accessing your cloud storage.\n"+
		"[#FFD700::b] -[-::-] [#FFD700::b]%s[-::-] This is your secret password for accessing your cloud storage.\n",
		strings.Trim(accessLabel, "🔑 "), strings.Trim(secretLabel, "🔑 "))

	if storageProtocol == "azstorage" {
		instructionsText += "[#FFD700::b] -[-::-] [#FFD700::b]Container Name:[-::-] This is the name of your Azure Blob Storage container.\n"
	}

	instructionsText += "\n[darkmagenta::i]\t\t\t*Keep these credentials secure. Do not share.[-]"

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetTextAlign(tuiAlignment).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Access key field widget
	accessKeyFieldWidget := tview.NewInputField().
		SetLabel(accessLabel).
		SetText(accessKey).
		SetFieldWidth(50).
		SetChangedFunc(func(key string) {
			accessKey = key
			accountName = key
		}).
		SetPlaceholder("\t\t\t\t<ENTER KEY HERE>").
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

	// Secret key field widget with masked input
	secretKeyFieldWidget := tview.NewInputField().
		SetLabel(secretLabel).
		SetText(string(secretKey)).
		SetFieldWidth(50).
		SetChangedFunc(func(key string) {
			secretKey = key
			accountKey = key
		}).
		SetPlaceholder("\t\t\t\t<ENTER KEY HERE>").
		SetMaskCharacter('*').
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

	// Container name field widget for Azure storage
	containerNameFieldWidget := tview.NewInputField().
		SetLabel("🪣 Container Name: ").
		SetText(containerName).
		SetPlaceholder("\t\t\t\t<ENTER NAME HERE>").
		SetChangedFunc(func(name string) {
			containerName = name
			bucketName = name
		}).
		SetFieldWidth(50).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(navigationHomeLabel, func() {
			pages.SwitchToPage("home")
		}).
		AddButton(navigationNextLabel, func() {
			// TODO: Add validation for access key and secret key HERE
			// For now, just check that they are not empty
			if (storageProtocol == "s3storage" && (len(accessKey) == 0 || len(secretKey) == 0)) ||
				(storageProtocol == "azstorage" && (len(accountName) == 0 || len(accountKey) == 0 || len(containerName) == 0)) {
				showErrorModal(
					app,
					pages,
					"[red::b]ERROR: Credential fields cannot be empty.\nPlease try again.[-::-]",
					func() {
						pages.SwitchToPage("page3")
					},
				)
				return
			}
			// TODO: Fix bug here where calling listBuckets() in the checkCredentials() function
			// causes the layout to shift upwards and the widgets to be misaligned if the user incorrectly
			// enters credentials.
			if err := checkCredentials(app, pages); err != nil {
				showErrorModal(app, pages, fmt.Sprintf("[red::b]ERROR: %s", err.Error()), func() {
					pages.RemovePage("page3")                  // Remove the current page
					page3 := buildCredentialsPage(app, pages)  // Rebuild the page
					pages.AddPage("page3", page3, true, false) // Add the new page
					pages.SwitchToPage("page3")
				})
				return
			}

			if storageProtocol == "azstorage" {
				pages.RemovePage("page4") // Remove previous page if it exists
				pages.SwitchToPage("page5")
			} else {
				page4 := buildBucketSelectionPage(app, pages)
				pages.AddPage("page4", page4, true, false)
				pages.SwitchToPage("page4")
			}
		}).
		AddButton(navigationBackLabel, func() {
			if storageProvider == "Microsoft" || storageProvider == "AWS" {
				pages.RemovePage("page2")
				pages.SwitchToPage("page1")
			} else {
				page2 := buildEndpointURLPage(app, pages)
				pages.AddPage("page2", page2, true, false)
				pages.SwitchToPage("page2")
			}
		}).
		AddButton(navigationPreviewLabel, func() {
			previewPage := buildPreviewPage(app, pages, "page3")
			pages.AddPage("previewPage", previewPage, true, false)
			pages.SwitchToPage("previewPage")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetLabelColor(widgetLabelColor).
		SetButtonBackgroundColor(navigationButtonColor).
		SetButtonTextColor(tcell.ColorBlack).
		SetButtonsAlign(navigationButtonAlignment)

	// Combine all credential widgets into a single form
	credentialsWidget := tview.NewForm().
		AddFormItem(accessKeyFieldWidget).
		AddFormItem(secretKeyFieldWidget).
		SetFieldTextColor(tcell.ColorBlack).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor)

	// If Azure is selected, add the container name field
	if storageProvider == "Microsoft" {
		credentialsWidget.AddFormItem(containerNameFieldWidget)
	}

	// Assemble page layout
	layout.SetDirection(tview.FlexRow)
	layout.AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false)
	layout.AddItem(nil, 1, 0, false)
	layout.AddItem(credentialsWidget, credentialsWidget.GetFormItemCount()*2+1, 0, false)
	layout.AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false)
	layout.AddItem(nil, 1, 0, false)
	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 4: Bucket Name Selection ---
//
// Function to build the bucket selection page. Allows users to select a bucket from a dropdown list
// of retrieved buckets based on provided s3 credentials. For s3 storage users only. Azure storage users will skip this page.
func buildBucketSelectionPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	instructionsText := "[#6EBE49::b] Select Your Bucket Name[-::-]\n" +
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n" +
		"[white::b] Select the name of your storage bucket from the dropdown below.[-::-]\n\n" +
		"[grey::i] The list of available buckets is retrieved from your cloud storage provider\n " +
		"based on the credentials provided in the previous step.[-::-]"

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetTextAlign(tuiAlignment).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Dropdown widget for selecting bucket name
	bucketSelectionWidget := tview.NewDropDown().
		SetLabel(" 🪣 Bucket Name: ").
		SetOptions(bucketList, func(name string, index int) {
			bucketName = name
		}).
		SetCurrentOption(0).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack).
		SetFieldWidth(25)

	// Navigation buttons widget
	navigationButtonsWidget := tview.NewForm().
		AddButton(navigationHomeLabel, func() {
			pages.SwitchToPage("home")
		}).
		AddButton(navigationNextLabel, func() {
			pages.SwitchToPage("page5")
		}).
		AddButton(navigationBackLabel, func() {
			pages.SwitchToPage("page3")
		}).
		AddButton(navigationPreviewLabel, func() {
			previewPage := buildPreviewPage(app, pages, "page4")
			pages.AddPage("previewPage", previewPage, true, false)
			pages.SwitchToPage("previewPage")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(navigationButtonColor).
		SetButtonTextColor(tcell.ColorBlack).
		SetButtonsAlign(navigationButtonAlignment)

	// Assemble page layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(bucketSelectionWidget, 2, 0, false).
		AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false).
		AddItem(nil, 1, 0, false)

	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Page 5: Caching Settings ---
//
// Function to build the caching page that allows users to configure caching settings.
// Includes options for enabling/disabling caching, specifying cache location, size, and retention settings.
func buildCachingPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	// Main layout container. Must be instantiated first to allow nested items.
	layout := tview.NewFlex().SetDirection(tview.FlexRow)

	instructionsText := "[#6EBE49::b] Configure Caching Settings[-]\n" +
		"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" +
		"[white::b] CloudFuse can cache data locally. You control the location, size, and duration.[-::-]\n\n" +
		"[#FFD700::b]  -[-::-] [#6EBE49::b]Enable[-::-] caching if you frequently re-read data and have ample disk space.\n" +
		"[#FFD700::b]  -[-::-] [red::b]Disable[-::-] caching if you prefer faster initial access or have limited disk space.\n\n"

	// Instructions text widget
	instructionsTextWidget := tview.NewTextView().
		SetTextAlign(tuiAlignment).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(instructionsText)

	// Dropdown widget for enabling/disabling caching
	cacheLocationFieldWidget := tview.NewInputField().
		SetLabel("📁 Cache Location: ").
		SetText(cacheLocation).
		SetFieldWidth(40).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack).
		SetChangedFunc(func(text string) {
			cacheLocation = text
		})

		// Input field widget for cache size percentage
	cacheSizeFieldWidget := tview.NewInputField().
		SetLabel("📊 Cache Size (%): ").
		SetText(cacheSize). // Default to 80%
		SetFieldWidth(4).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack).
		SetChangedFunc(func(text string) {
			if size, err := strconv.Atoi(text); err != nil || size < 1 || size > 100 {
				showErrorModal(
					app,
					pages,
					"[red::b]ERROR: Cache size must be between 1 and 100.\nPlease try again.[-::-]",
					func() {
						pages.SwitchToPage("page5")
					},
				)
				return
			}
			cacheSize = text
		})

	// Input field widget for cache retention duration
	cacheRetentionDurationFieldWidget := tview.NewInputField().
		SetLabel("⌛ Cache Retention Duration: ").
		SetText(fmt.Sprintf("%d", cacheRetentionDuration)).
		SetFieldWidth(5).
		SetChangedFunc(func(text string) {
			if val, err := strconv.Atoi(text); err == nil {
				cacheRetentionDuration = val
			} else {
				// TODO: Handle invalid input
				cacheRetentionDuration = 0
			}
		}).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

	// Dropdown widget for cache retention unit
	cacheRetentionUnitDropdownWidget := tview.NewDropDown().
		SetOptions([]string{"Seconds", "Minutes", "Hours", "Days"}, func(option string, index int) {
			cacheRetentionUnit = option
			// Convert cache retention duration to seconds
			switch cacheRetentionUnit {
			case "Seconds":
				cacheRetentionDurationSec = cacheRetentionDuration
			case "Minutes":
				minutes := cacheRetentionDuration
				cacheRetentionDurationSec = minutes * 60
			case "Hours":
				hours := cacheRetentionDuration
				cacheRetentionDurationSec = hours * 3600
			case "Days":
				days := cacheRetentionDuration
				cacheRetentionDurationSec = days * 86400
			}
		}).
		SetCurrentOption(3). // Default to Days
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

		// Dropdown widget for enabling/disabling cache cleanup on restart
		// If enabled --> allow-non-empty-temp: false
		// if disabled --> allow-non-empty-temp: true
	clearCacheOnStartDropdownWidget := tview.NewDropDown().
		SetLabel("🧹 Clear Cache On Start: ").
		SetOptions([]string{" Enabled ", " Disabled "}, func(text string, index int) {
			if text == " Enabled " {
				clearCacheOnStart = true
			} else {
				clearCacheOnStart = false
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

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
		AddButton(navigationHomeLabel, func() {
			pages.SwitchToPage("home")
		}).
		AddButton(navigationFinishLabel, func() {
			// Check if caching is enabled and validate cache settings
			if enableCaching {
				// Validate the cache location
				if err := validateCachePath(); err != nil {
					showErrorModal(app, pages, "Invalid cache location:\n"+err.Error(), func() {
						pages.SwitchToPage("page5")
					})
					return
				}

				// Check available cache size
				if err := getAvailableCacheSize(); err != nil {
					showErrorModal(
						app,
						pages,
						"Failed to check available cache size:\n"+err.Error(),
						func() {
							pages.SwitchToPage("page5")
						},
					)
					return
				}

				cacheSizeText := fmt.Sprintf(
					"Available Disk Space @ Cache Location: [darkred::b]%d GB[-::-]\n",
					availableCacheSizeGB,
				) +
					fmt.Sprintf(
						"Cache Size Currently Set to: [darkred::b]%.0f GB (%s%%)[-::-]\n\n",
						float64(currentCacheSizeGB),
						cacheSize,
					) +
					"Would you like to proceed with this cache size?\n\n" +
					"If not, hit [darkred::b]Return[-::-] to adjust cache size accordingly. Otherwise, hit [darkred::b]Finish[-::-] to complete the configuration."

				showCacheConfirmationModal(app, pages, cacheSizeText,
					// Callback function if the user selects Finish
					func() {
						if err := createYAMLConfig(); err != nil {
							showErrorModal(
								app,
								pages,
								"Failed to create YAML config:\n"+err.Error(),
								func() {
									pages.SwitchToPage("page5")
								},
							)
							return
						}
						showExitModal(app, pages, func() {
							app.Stop()
						})
					},
					// Callback function if the user selects Return
					func() {
						pages.SwitchToPage("page5")
					})

			} else {
				// If caching is disabled, just finish the configuration
				if err := createYAMLConfig(); err != nil {
					showErrorModal(app, pages, "Failed to create YAML config:\n"+err.Error(), func() {
						pages.SwitchToPage("page5")
					})
					return
				}
				showExitModal(app, pages, func() {
					app.Stop()
				})
			}
		}).
		AddButton(navigationBackLabel, func() {
			if storageProtocol == "azstorage" {
				pages.SwitchToPage("page3")
			} else {
				page4 := buildBucketSelectionPage(app, pages)
				pages.AddPage("page4", page4, true, false)
				pages.SwitchToPage("page4")
			}
		}).
		AddButton(navigationPreviewLabel, func() {
			previewPage := buildPreviewPage(app, pages, "page5")
			pages.AddPage("previewPage", previewPage, true, false)
			pages.SwitchToPage("previewPage")
		}).
		AddButton(navigationQuitLabel, func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(navigationButtonColor).
		SetButtonTextColor(tcell.ColorBlack).
		SetButtonsAlign(tuiAlignment)

		// Widget to enable/disable caching
	enableCachingDropdownWidget := tview.NewDropDown()
	enableCachingDropdownWidget.
		SetLabel("💾 Caching: ").
		SetOptions([]string{" Enabled ", " Disabled "}, func(text string, index int) {
			if text == " Enabled " {
				cacheMode = "file_cache"
				enableCaching = true
				if !showCacheFields {
					layout.RemoveItem(navigationButtonsWidget)
					layout.RemoveItem(cacheFields)
					layout.AddItem(cacheFields, 8, 0, false)
					layout.AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false)
					showCacheFields = true
				}
			} else {
				cacheMode = "block_cache"
				enableCaching = false
				if showCacheFields {
					layout.RemoveItem(cacheFields)
					showCacheFields = false
				}
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(widgetLabelColor).
		SetFieldBackgroundColor(widgetFieldBackgroundColor).
		SetFieldTextColor(tcell.ColorBlack)

		// Assemble page layout
	layout.AddItem(instructionsTextWidget, getTextHeight(instructionsText), 0, false)
	layout.AddItem(enableCachingDropdownWidget, 2, 0, false)

	if showCacheFields {
		layout.AddItem(cacheFields, 8, 0, false)
	}

	layout.AddItem(navigationButtonsWidget, navigationWidgetHeight, 0, false)
	layout.AddItem(nil, 1, 0, false)
	layout.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return layout
}

//	--- Summary Page ---
//
// Function to build the summary page that displays the configuration summary.
// This function creates a text view with the summary information and a return button.
// The preview page parameter allows switching back to the previous page when the user clicks "Return".
func buildPreviewPage(
	app *tview.Application,
	pages *tview.Pages,
	previewPage string,
) tview.Primitive {
	summaryText :=
		"[#6EBE49::b] CloudFuse Summary Configuration:[-]\n" +
			"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n[-]" +
			fmt.Sprintf(" Storage Provider: [#FFD700::b]%s[-]\n", storageProvider) +
			fmt.Sprintf("     Endpoint URL: [#FFD700::b]%s[-]\n", endpointURL) +
			fmt.Sprintf("      Bucket Name: [#FFD700::b]%s[-]\n", bucketName) +
			fmt.Sprintf("       Cache Mode: [#FFD700::b]%s[-]\n", cacheMode) +
			fmt.Sprintf("   Cache Location: [#FFD700::b]%s[-]\n", cacheLocation) +
			fmt.Sprintf(
				"       Cache Size: [#FFD700::b]%s%% (%d GB)[-]\n",
				cacheSize,
				currentCacheSizeGB,
			)

	// Display cache retention duration in seconds and specified unit
	if cacheRetentionUnit == "Seconds" {
		summaryText += fmt.Sprintf(
			"  Cache Retention: [#FFD700::b]%d Seconds[-]\n\n",
			cacheRetentionDurationSec,
		)
	} else {
		summaryText += fmt.Sprintf("  Cache Retention: [#FFD700::b]%d sec (%d %s)[-]\n\n",
			cacheRetentionDurationSec, cacheRetentionDuration, cacheRetentionUnit)
	}

	// Set a dynamic width and height for the summary widget
	summaryWidgetHeight := getTextHeight(summaryText)
	summaryWidgetWidth := getTextWidth(summaryText) / 3

	summaryWidget := tview.NewTextView().
		SetTextAlign(tuiAlignment).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(summaryText).
		SetScrollable(true)

	returnButton := tview.NewButton("[black]Return[-]").
		SetSelectedFunc(func() {
			pages.SwitchToPage(previewPage)
		})
	returnButton.SetBackgroundColor(greenColor)
	returnButton.SetBorder(true)
	returnButton.SetBorderColor(yellowColor)
	returnButton.SetBackgroundColorActivated(greenColor)

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

	leftAlignedModal.SetBorder(true).SetBorderColor(greenColor).SetBorderPadding(1, 1, 1, 1)

	return leftAlignedModal
}

// Function to show a modal dialog with a message and an "OK" button.
// This function is used to display error messages or confirmations.
// May specify a callback function to execute when the modal is closed.
func showErrorModal(app *tview.Application, pages *tview.Pages, message string, onClose func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("modal")
			onClose()
		}).
		SetBackgroundColor(greenColor).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(yellowColor)
	modal.SetButtonBackgroundColor(yellowColor)
	modal.SetButtonTextColor(tcell.ColorBlack)
	pages.AddPage("modal", modal, false, true)
}

// Function to show a confirmation modal dialog with "Finish" and "Return" buttons.
// Used to confirm cache size before proceeding. Must specify two callback functions for the "Finish" and "Return" actions.
func showCacheConfirmationModal(
	app *tview.Application,
	pages *tview.Pages,
	message string,
	onFinish func(),
	onReturn func(),
) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Finish", "Return"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("modal")
			if buttonLabel == "Finish" {
				onFinish()
			} else {
				onReturn()
			}
		}).
		SetBackgroundColor(greenColor).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(yellowColor)
	modal.SetButtonBackgroundColor(yellowColor)
	modal.SetButtonTextColor(tcell.ColorBlack)
	pages.AddPage("modal", modal, true, true)
}

// Function to show final exit modal when configuration is complete.
// Informs the user that the configuration is complete and they can exit.
// This function is called when the user clicks "Finish" on the caching page.
func showExitModal(app *tview.Application, pages *tview.Pages, onConfirm func()) {

	processingEmojis := []string{"🕐", "🕑", "🕒", "🕓", "🕔", "🕕", "🕖", "🕗", "🕘", "🕙", "🕚", "✅"}

	modal := tview.NewModal().
		AddButtons([]string{"Exit"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("modal")
			if buttonLabel == "Exit" {
				onConfirm()
			}
		}).
		SetBackgroundColor(greenColor).
		SetTextColor(tcell.ColorBlack)
	modal.SetBorder(true)
	modal.SetBorderColor(yellowColor)
	modal.SetButtonBackgroundColor(yellowColor)
	modal.SetButtonTextColor(tcell.ColorBlack)

	pages.AddPage("modal", modal, true, true)

	// Simulate processing with emoji animation
	go func() {
		// Show initial message with emoji animation
		for i := 0; i < len(processingEmojis); i++ {
			currentEmoji := processingEmojis[i]
			time.Sleep(100 * time.Millisecond)
			app.QueueUpdateDraw(func() {
				modal.SetText(
					fmt.Sprintf(
						"[#6EBE49::b]Creating configuration file...[-::-]\n\n%s",
						currentEmoji,
					),
				)
			})
		}

		// After animation, show final message
		app.QueueUpdateDraw(func() {
			modal.SetText(fmt.Sprintf("[#6EBE49::b]Configuration Complete![-::-]\n\n%s\n\n"+
				"Your CloudFuse configuration file has been created at:\n\n[blue:white:b]%s[-:-:-]\n\n"+
				"You can now exit the application.\n\n"+
				"[black::i]Thank you for using CloudFuse Config![-::-]", processingEmojis[len(processingEmojis)-1], configFilePath))
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
			"[red::b]ERROR: Failed to get home directory: %v\nUsing fallback path for cache directory.\n",
			err,
		)
		return getFallbackCachePath()
	}
	cachePath := filepath.Join(home, ".cloudfuse", "file_cache")
	// If the directory doesn't exist, create it
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		if err := os.MkdirAll(cachePath, 0700); err != nil {
			fmt.Printf(
				"[red::b]ERROR: Failed to create cache directory: %v\nUsing fallback path for cache directory.\n",
				err,
			)
			return getFallbackCachePath()
		}
	}
	// Return the full path to the cache directory
	return cachePath
}

// Helper function to validate the entered cache path.
func validateCachePath() error {
	// Validate that the path is not empty
	if strings.TrimSpace(cacheLocation) == "" {
		return fmt.Errorf("[red::b]ERROR: Cache location cannot be empty[-::-]")
	}
	// Make sure no invalid path characters are used
	if strings.ContainsAny(cacheLocation, `<>:"|?*#%^&;'"`+"`"+`{}[]`) {
		return fmt.Errorf("[red::b]ERROR: Cache location contains invalid characters[-::-]")
	}
	// Validate that the cache path exists
	if cacheLocation != getDefaultCachePath() && cacheMode == "file_cache" {
		if _, err := os.Stat(cacheLocation); os.IsNotExist(err) {
			return fmt.Errorf("[red::b]ERROR: '%s': No such file or directory[-::-]", cacheLocation)
		}
	}
	return nil
}

// Helper function to get the available disk space at the cache location and calculates
// the cache size in GB based on the user-defined cache size percentage.
func getAvailableCacheSize() error {
	availableBlocks, _, err := common.GetAvailFree(cacheLocation)
	if err != nil {
		// If we fail to get the available cache size, we default to 80% of the available disk space
		cacheSize = "80"
		returnMsg := fmt.Errorf(
			"[red::b]WARNING: Failed to get available cache size at '%s': %v\n\n"+
				"Defaulting cache size to 80%% of available disk space.\n\n"+
				"Please manually verify you have enough disk space available for caching.[-::-]",
			cacheLocation,
			err,
		)
		return returnMsg
	}

	const blockSize = 4096
	availableCacheSizeBytes := availableBlocks * blockSize                     // Convert blocks to bytes
	availableCacheSizeGB = int(availableCacheSizeBytes / (1024 * 1024 * 1024)) // Convert to GB
	cacheSizeInt, _ := strconv.Atoi(cacheSize)
	currentCacheSizeGB = int(availableCacheSizeGB) * cacheSizeInt / 100

	return nil
}

// Helper function to normalize and validate the user-defined endpoint URL.
func validateEndpointURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)

	// Check if the URL is empty
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("[red::b]Endpoint URL cannot be empty[-::-]\nPlease try again.")
	}

	// Normalize the URL by adding "https://" if it doesn't start with "http://" or "https://"
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		endpointURL = "https://" + rawURL
		return fmt.Errorf("[red::b]Endpoint URL should start with 'http://' or 'https://'.\n" +
			"Appending 'https://' to the URL...\n\nPlease verify the URL and try again.")
	}

	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return fmt.Errorf("[red::b]Invalid URL format[-::-]\n%s\nPlease try again.", err.Error())
	}

	return nil
}

// Function to create a temporary YAML configuration file based on user inputs.
// Used for testing credentials and then removed after the check.
// Called when the user clicks "Next" on the credentials page.
func createTmpConfigFile() error {
	config := configuration{

		Components: []string{storageProtocol},
	}

	if storageProtocol == "azstorage" {
		config.AzStorage = azstorage.AzStorageOptions{
			AccountType: "block",
			AccountName: accountName,
			AccountKey:  accountKey,
			AuthMode:    "key",
			Container:   containerName,
		}
	} else {
		config.S3Storage = s3StorageConfig{
			BucketName:      bucketName,
			KeyID:           accessKey,
			SecretKey:       secretKey,
			Endpoint:        endpointURL,
			EnableDirMarker: true,
		}

	}

	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %v", err)
	}

	tmpFile := "config-tmp.yaml"
	if err := os.WriteFile(tmpFile, yamlData, 0600); err != nil {
		return fmt.Errorf("failed to write YAML to file: %v", err)
	}

	// Update options.ConfigFile to point to the temporary file
	options.ConfigFile = "config-tmp.yaml"
	return nil
}

// Function to check the credentials entered by the user.
// Attempts to connect to the storage backend and fetch the bucket list.
// If successful, populates the global `bucketList` variable with the list of available buckets (for s3 providers only).
// Called when the user clicks "Next" on the credentials page.
func checkCredentials(app *tview.Application, pages *tview.Pages) error {
	// Create a temporary config file for testing credentials
	if err := createTmpConfigFile(); err != nil {
		return fmt.Errorf("Failed to create temporary config file: %v", err)
	}

	// Delete the temporary config file regardless of success or failure of the credential check
	defer func() {
		_ = os.Remove("config-tmp.yaml")
	}()

	// Parse and unmarshal the temporary config file
	if err := parseConfig(); err != nil {
		return fmt.Errorf("Failed to parse config: %v", err)
	}

	if err := config.Unmarshal(&options); err != nil {
		return fmt.Errorf("Failed to unmarshal config: %v", err)
	}

	// Try to fetch bucket list
	var err error
	if slices.Contains(options.Components, "azstorage") {
		bucketList, err = getContainerListAzure()

	} else if slices.Contains(options.Components, "s3storage") {
		bucketList, err = getBucketListS3()

	} else {
		err = fmt.Errorf("Unsupported storage backend")
	}

	if err != nil {
		return fmt.Errorf("Failed to get bucket list: %v", err)
	}

	return nil
}

// Function to create the YAML configuration file based on user inputs once all forms are completed.
// Called when the user clicks "Finish" on the caching page.
func createYAMLConfig() error {
	config := configuration{
		Components: []string{"libfuse", cacheMode, "attr_cache", storageProtocol},

		Libfuse: libfuse.LibfuseOptions{
			NetworkShare: true,
		},

		AttrCache: attr_cache.AttrCacheOptions{
			Timeout: uint32(7200),
		},
	}

	if cacheMode == "file_cache" {
		config.FileCache = file_cache.FileCacheOptions{
			TmpPath:       cacheLocation,
			Timeout:       uint32(cacheRetentionDurationSec),
			AllowNonEmpty: !clearCacheOnStart,
			SyncToFlush:   true,
		}
		// If cache size is not set to 80%, convert currentCacheSizeGB to MB and set file_cache.max-size-mb to it
		if cacheSize != "80" {
			config.FileCache.MaxSizeMB = float64(currentCacheSizeGB * 1024) // Convert GB to MB
		}
	}

	if storageProtocol == "s3storage" {
		config.S3Storage = s3StorageConfig{
			BucketName:      bucketName,
			KeyID:           accessKey,
			SecretKey:       secretKey,
			Endpoint:        endpointURL,
			EnableDirMarker: true,
		}
	} else {
		config.AzStorage = azstorage.AzStorageOptions{
			AccountType: "block",
			AccountName: accountName,
			AccountKey:  accountKey,
			AuthMode:    "key",
			Container:   containerName,
		}
	}

	// Marshal the struct to YAML (returns []byte and error)
	yamlData, err := yaml.Marshal(&config)
	if err != nil {
		return fmt.Errorf("Failed to marshal YAML: %v", err)
	}

	// Write the YAML to a file
	if err := os.WriteFile("config.yaml", yamlData, 0600); err != nil {
		return fmt.Errorf("Failed to write YAML to file: %v", err)
	}

	// Update global configFilePath variable
	currDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error: %v", err)
	}

	configFilePath = filepath.Join(currDir, "config.yaml")

	return nil
}
