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
	"slices"
	"strconv"
	"strings"

	"github.com/Seagate/cloudfuse/common/config"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"gopkg.in/yaml.v3"
)


var (
	accountName = "my-account" 
	storageProtocols = []string {"s3storage", "azstorage"}
	storageProtocol = "s3storage" 
	storageProviders = []string{"LyveCloud", "Microsoft", "AWS", "Other"}
	storageProvider = "LyveCloud" 
	cacheModes = []string {"stream", "file_cache", "block_cache"} 
	cacheMode = "file_cache" 
	bucketName = "my-bucket" 
	containerList = []string {}
	cacheLocation = "/var/cache/s3storage" 
	cacheSize = "80" 
	cacheRetentionDuration = "30" 
	cacheRetentionUnit = "Days"
	endpointURL = "https://s3.sv15.seagate.com"
	region = "us-east-1"
	previewPage = "page1"
	accessKey = ""
	secretKey = ""
	menuButtonColor = tcell.GetColor("#6EBE49")
	menuButtonTextColor = tcell.ColorBlack
	menuButtonAlignment = tview.AlignLeft
)


type Config struct {
    Logging    LoggingConfig       `yaml:"logging"`
    Components []string            `yaml:"components"`
    Libfuse    LibfuseConfig       `yaml:"libfuse"`
    Stream     StreamConfig        `yaml:"stream,omitempty"`
	FileCache  FileCacheConfig     `yaml:"file_cache,omitempty"`
	BlockCache BlockCacheConfig    `yaml:"block_cache,omitempty"`
    AttrCache  AttrCacheConfig     `yaml:"attr_cache"`
    S3Storage  S3StorageConfig 	   `yaml:"s3storage"`
	AzStorage  *AzureStorageConfig `yaml:"azstorage,omitempty"` 
}

type LoggingConfig struct {
    Type  string `yaml:"type"`
    Level string `yaml:"level"`
}

type LibfuseConfig struct {
    AttributeExpirationSec     int  `yaml:"attribute-expiration-sec"`
    EntryExpirationSec         int  `yaml:"entry-expiration-sec"`
    NegativeEntryExpirationSec int  `yaml:"negative-entry-expiration-sec"`
    NetworkShare               bool `yaml:"network-share"`
}

type StreamConfig struct {
    BlockSizeMB   int `yaml:"block-size-mb"`
    BlocksPerFile int `yaml:"blocks-per-file"`
    CacheSizeMB   int `yaml:"cache-size-mb"`
}

type FileCacheConfig struct {
	Path 			string 	`yaml:"path"`
	TimeOutSec 		int 	`yaml:"timeout-sec"`
	CleanUpOnStart	bool 	`yaml:"cleanup-on-start"`
	IgnoreSync 		bool 	`yaml:"ignore-sync"`
}

type BlockCacheConfig struct {
	BlockSizeMB 	int `yaml:"block-size-mb"`
	MemorySizeMB 	int `yaml:"mem-size-mb"`
	Prefetch 		int `yaml:"prefetch"`
	Parallelism 	int `yaml:"parallelism"`
}


type AttrCacheConfig struct {
    TimeoutSec int `yaml:"timeout-sec"`
}

type S3StorageConfig struct {
    BucketName       string `yaml:"bucket-name"`
    KeyID            string `yaml:"key-id"`
    SecretKey        string `yaml:"secret-key"`
    Endpoint         string `yaml:"endpoint"`
    Region           string `yaml:"region"`
    EnableDirMarker  bool   `yaml:"enable-dir-marker"`
}


type AzureStorageConfig struct {
	Type 			 string `yaml:"type"`
    AccountName      string `yaml:"account-name"`
    AccountKey       string `yaml:"account-key"`
    Endpoint         string `yaml:"endpoint"`
    Mode             string `yaml:"mode"`
    Container  		 string `yaml:"container"`
}


func runTUI() error{
	app := tview.NewApplication()
	app.EnableMouse(true)
	app.EnablePaste(true)

	buildTUI(app)

	if err := app.Run(); err != nil {
		panic(err)
	}

	// After the TUI is done, create the YAML config file
	createYAMLConfig()

	return nil
}


func buildTUI(app *tview.Application) {
	pages := tview.NewPages()

	// --- Home Page ---
	homePage := buildHomePage(app, pages)

	// --- Page 1: Storage Type Selection ---
	page1 := buildStorageSelectionPage(app, pages)

	// --- Page 2: Endpoint & Region Entry ---
	page2 := buildEndpointRegionPage(app, pages)

	// --- Page 3: Credentials Entry ---
	page3 := buildCredentialsPage(app, pages)

	// --- Page 4: Bucket Name Entry ---
	page4 := buildContainerSelectPage(app, pages)

	// --- Page 5: Caching Settings ---
	page5 := buildCachingPage(app, pages)

	// --- Add pages to the page stack ---
	pages.AddPage("home", homePage, true, true)
	pages.AddPage("page1", page1, true, false)
	pages.AddPage("page2", page2, true, false)
	pages.AddPage("page3", page3, true, false)
	pages.AddPage("page4", page4, true, false)
	pages.AddPage("page5", page5, true, false)

	app.SetRoot(pages, true)
}


func buildHomePage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	// Banner / welcome message
	bannerText := "[#6EBE49::b]░█▀▀░█░░░█▀█░█░█░█▀▄░█▀▀░█░█░█▀▀░█▀▀\n" +
							  "░█░░░█░░░█░█░█░█░█░█░█▀▀░█░█░▀▀█░█▀▀\n" +
							  "░▀▀▀░▀▀▀░▀▀▀░▀▀▀░▀▀░░▀░░░▀▀▀░▀▀▀░▀▀▀[-]\n\n" +
					"[white::b]Welcome to the CloudFuse Configuration Tool\n" + 
					"[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n" + 
					"[#6EBE49::b]Cloud storage configuration made easy via terminal.[-]\n\n" + 
					"[::b]Press [#FFD700]Start[-] to begin or [red]Quit[-] to exit.\n"
	
	bannerView := tview.NewTextView().
		SetText(bannerText).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetWrap(true)

	// Instructions
	instructionsView := tview.NewTextView().
		SetText("[::b]Instructions:[-:-]\n" +
  				"[#6EBE49]•[-] Use your [::b]mouse[-:-] or [::b]arrow keys[-:-] to navigate.\n" +
  				"[#6EBE49]•[-] Press [::b]Enter[-:-] to select items.\n" +
  				"[#6EBE49]•[-] For the best experience, expand terminal window to full size.\n").
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetWrap(true)

	// Dropdown hint
	jumpToView := tview.NewTextView().
		SetText("[::i]Tip: Use the dropdown below to quickly jump to any step.[::-]").
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetWrap(true)

	// Start / Quit buttons
	startQuitWidget := tview.NewForm().
		AddButton("🚀 Start", func() {
			pages.SwitchToPage("page1")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(tcell.GetColor("#6EBE49")).
		SetButtonTextColor(tcell.ColorWhite).
		SetButtonsAlign(tview.AlignCenter)

	// Dropdown to jump between pages
	jumpToWidget := tview.NewForm().
		AddDropDown("Jump to:", []string{
			" Storage Selection ⬇️ ",
			" Endpoint & Region ",
			" Credentials ",
			" Bucket Name ",
			" Caching Settings ",
		}, 0, func(option string, index int) {
			pages.SwitchToPage(fmt.Sprintf("page%d", index+1))
		}).
		SetLabelColor(tcell.GetColor("#FFD700")).
		SetFieldBackgroundColor(tcell.GetColor("#FFD700"))

		// About section
	aboutView := tview.NewTextView().
		SetText("[::b]ABOUT[-]\n" +
				"[gray]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[-]\n\n" +
				"CloudFuse TUI Configuration Tool\n\n" +
				"Seagate Technology, LLC\n" + 
				"cloudfuse@seagate.com\n\n" +
				"Version: 1.0.0").

		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetWrap(true)

	// Assemble layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).                // Top padding
		AddItem(bannerView, 10, 0, false).		  // Banner
		AddItem(nil, 1, 0, false).				  // Banner and start/quit padding
		AddItem(startQuitWidget, 3, 0, false).	  // Start/Quit buttons
		AddItem(nil, 1, 0, false).		          // Padding between buttons and instructions
		AddItem(instructionsView, 4, 0, false).   // Instructions
		AddItem(nil, 2, 0, false).				  // Padding between instructions and dropdown hint
		AddItem(jumpToView, 1, 0, false).
		AddItem(jumpToWidget, 3, 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(aboutView, 9, 0, false).          // New About section
		AddItem(nil, 1, 0, false)                 // Bottom padding
	layout.SetBorder(true).SetBorderColor(tcell.GetColor("#6EBE49")).SetBorderPadding(1, 1, 1, 1)

	return layout
}


func buildStorageSelectionPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	// Header / section banner
	headerText := "[#6EBE49::b]Step 1: Select Your Cloud Storage Provider[-]\n" +
			  "[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n" + 
			  "[white]Choose your cloud storage provider from the dropdown below.\n\n" +
			  "If your provider is not listed, choose [::b]Other[::-] and you’ll be prompted " +
			  "to enter the endpoint URL and region manually."

	pageText := tview.NewTextView().
		SetText(headerText).
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetWrap(true)

	// Dropdown for storage provider
	storageProviderDropdown := tview.NewDropDown().
		SetLabel("📦 Storage Provider: ").
		SetOptions([]string{" LyveCloud ⬇️", " Microsoft ", " AWS ", " Other "}, func(option string, index int) {
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
			case " Other ":
				storageProtocol = "s3storage"
				storageProvider = "Other"
			default:
				storageProtocol = "s3storage"
				storageProvider = "LyveCloud" 
			}
		}).
		SetCurrentOption(0).
		SetLabelColor(tcell.GetColor("#FFD700")).
		SetFieldBackgroundColor(tcell.GetColor("#FFD700")).
		SetFieldWidth(14)
		

	// Navigation buttons
	form := tview.NewForm().
		// AddFormItem(storageProviderDropdown).
		AddButton("🏠 Home", func() {
			pages.SwitchToPage("home")
		}).
		AddButton("➡ Next", func() {
			page2 := buildEndpointRegionPage(app, pages)
			pages.AddPage("page2", page2, true, false)
			pages.SwitchToPage("page2")
		}).
		AddButton("📄 Preview", func() {
			summaryPage := buildSummaryPage(app, pages)
			pages.AddPage("summaryPage", summaryPage, true, false)
			pages.SwitchToPage("summaryPage")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(menuButtonColor).
		SetButtonTextColor(menuButtonTextColor).
		SetButtonsAlign(tview.AlignCenter)

	// Layout assembly
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).             // Top padding
		AddItem(pageText, 7, 0, false).        // Header and instructions
		AddItem(nil, 1, 0, false).             // Spacing
		AddItem(storageProviderDropdown, 3, 0, false). // Dropdown for storage provider
		AddItem(form, 6, 0, false).             // Dropdown + nav buttons
		AddItem(nil, 1, 0, false)              // Bottom padding

	return layout
}


// func buildEndpointRegionPage(app *tview.Application, pages *tview.Pages) tview.Primitive {

// 	var regions []string
// 	var regionInput tview.FormItem
	
// 	// Switch case to set the default URL and region based on the selected storage provider
// 	switch storageProvider {
// 	case "LyveCloud":
// 		urlRegionHelpText = "For LyveCloud, the endpoint URL general format is: \n\n" +
// 							"\t[darkmagenta::b]https://s3.<[-][darkcyan::b]region[-][darkmagenta::b]>.lyvecloud.seagate.com[-]\n\n" + 
// 							"For example, if your region is \"us-east-1\", the endpoint URL would be:\n\n" +
// 							"\t[darkmagenta::b]https://s3.us-east-1.seagate.com[-]\n\n" +
// 							"You can also use the LyveCloud portal to find your storage account endpoint.\n" +
// 							"The available regions for Seagate LyveCloud are listed in the dropdown below."
// 		urlText = "https://s3.sv15.seagate.com"
// 		regionText = "us-east-1"
// 		regions = lyvecloudRegions
// 	case "Microsoft":
// 		urlRegionHelpText = "For Microsoft Azure, the endpoint URL general format is:\n\n" +
// 							"\t[darkmagenta::b]https://<[-][darkcyan::b]storage-account-name[-][darkmagenta::b]>.<[-][darkcyan::b]service[-][darkmagenta::b]>.core.windows.net[-]\n\n" +
// 							"For example, if your storage account name is \"mystorageaccount\" and the\n" +
// 							"service is \"file\", the endpoint URL would be:\n\n" +
// 							"\t[darkmagenta::b]https://mystorageaccount.file.core.windows.net[-]\n\n" +
// 							"You can also use the Azure portal to find your storage account endpoint.\n" +
// 							"The available regions for Microsoft Azure are listed in the dropdown below."
// 		urlText = "https://<account>.file.core.windows.net"
// 		regionText = "us-east"
// 		regions = azureRegions
// 	case "AWS":
// 		urlRegionHelpText = "For AWS S3, the endpoint URL general format is:\n\n" +
// 							"\t[darkmagenta::b]https://s3.<[-][darkcyan::b]region[-][darkmagenta::b]>.amazonaws.com[-]\n\n" +
// 							"For example, if your region is \"us-east-1\", the endpoint URL would be:\n\n" +
// 							"\t[darkmagenta::b]https://s3.us-east-1.amazonaws.com[-]\n\n" +
// 							"You can also use the AWS Management Console to find your S3 bucket endpoint.\n" +
// 							"The available regions for AWS S3 are listed in the dropdown below."
// 		urlText = "https://s3.amazonaws.com"
// 		regionText = "us-east-1"
// 		regions = awsRegions
// 	case "Other":
// 		urlText = "https://your-storage-endpoint.com"
// 		regionText = "your-region" // Default for 'Other'
// 	default:
// 		urlText = "https://s3.sv15.seagate.com" // Default for LyveCloud
// 		regionText = "us-east-1" // Default region
// 	}

// 	pageText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(fmt.Sprintf("[green::b]Endpoint URL and Region for %s:[-] \n\n%s", storageProvider, urlRegionHelpText))
	
// 	urlInput := tview.NewInputField().
// 		SetLabel("Endpoint URL:").
// 		SetText(urlText).
// 		SetFieldWidth(60).
// 		SetChangedFunc(func(text string) {
// 			urlText = text
// 		})

// 	// Dropdown for region selection based on storage type
// 	// var regions []string
// 	// var regionInput tview.FormItem

// 	if storageProvider != "Other" {
// 		regionInput = tview.NewDropDown().
// 			SetLabel("Region:").
// 			SetOptions(regions, func(text string, index int) {
// 				regionText = text
// 			}).
// 			SetCurrentOption(0)

// 	} else {
// 		regionInput = tview.NewInputField().
// 			SetLabel("Region:").
// 			SetText("Enter Region (i.e., us-east-1)").
// 			SetFieldWidth(30).
// 			SetChangedFunc(func(text string) {
// 				regionText = text
// 			})
// 		}

// 	form := tview.NewForm().
// 		AddFormItem(urlInput).
// 		AddFormItem(regionInput).
// 		AddButton("Home", func() {
// 			pages.SwitchToPage("home")
// 		}).
// 		AddButton("Next", func() {
// 			// Normalize and validate the URL
// 			_, err := validateURL(urlText)
// 			if err != nil {
// 				showModal(app, pages, "Invalid URL format.\nPlease try again.", func() {
// 					pages.SwitchToPage("page2")
// 				})
// 				return
// 			}
		
// 			// Move to the next page
// 			pages.SwitchToPage("page3")
// 		}).
// 		AddButton("Back", func() {
// 			pages.SwitchToPage("page1")
// 		}).
// 		AddButton("Preview", func() {
// 			// Normalize and validate URL
// 			_, err := validateURL(urlText)
// 			if err != nil {
// 				showModal(app, pages, "Invalid URL format.\nPlease try again.", func() {
// 					pages.SwitchToPage("page2")
// 				})
// 				return
// 			}
			
// 			previewPage = "page2"
// 			summaryPage := buildSummaryPage(app, pages)
// 			pages.AddPage("summaryPage", summaryPage, true, false)
// 			pages.SwitchToPage("summaryPage") // Switch to Page 3
// 		}).
// 		AddButton("Quit", func() {
// 			app.Stop()
// 		})

// 	// form.SetBorder(true).SetTitle("[ Endpoint and Region ]").SetTitleAlign(tview.AlignLeft)
// 	form.SetButtonBackgroundColor(menuButtonColor).SetButtonTextColor(menuButtonTextColor)

// 	layout := tview.NewFlex().
// 		SetDirection(tview.FlexRow).
// 		AddItem(nil, 1, 1, false).           	// Top padding
// 		AddItem(pageText, 13, 1, false).      	// Main content
// 		AddItem(form, 10, 1, false).           	// Form for endpoint
// 		AddItem(nil, 1, 1, false)             	// Bottom padding

// 	return layout
// }



func buildEndpointRegionPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	var regions []string
	var regionInput tview.FormItem
	urlRegionHelpText := ""

	// Determine URL, region, and help text based on selected provider
	switch storageProvider {
	case "LyveCloud":
		urlRegionHelpText = `[::b]For LyveCloud, the endpoint URL format is:[-]

  [darkmagenta::b]  https://s3.<[darkcyan::b]region[darkmagenta::b]>.lyvecloud.seagate.com[-]

Example:
  [darkmagenta::b]  https://s3.us-east-1.seagate.com[-]

Find more info in your LyveCloud portal.
Available regions are shown in the dropdown below.`
		endpointURL = "https://s3.sv15.seagate.com"
		region = "us-east-1"
		regions = lyvecloudRegions

	case "Microsoft":
		urlRegionHelpText = `[::b]For Microsoft Azure, the endpoint URL format is:[-]

  [darkmagenta::b]  https://<[darkcyan::b]account-name[darkmagenta::b]>.<[darkcyan::b]service[darkmagenta::b]>.core.windows.net[-]

Example:
  [darkmagenta::b]  https://mystorageaccount.file.core.windows.net[-]

Find more info in the Azure portal.
Available regions are listed below.`
		endpointURL = "https://<account>.file.core.windows.net"
		region = "us-east"
		regions = azureRegions

	case "AWS":
		urlRegionHelpText = `[::b]For AWS S3, the endpoint URL format is:[-]

  [darkmagenta::b]  https://s3.<[darkcyan::b]region[darkmagenta::b]>.amazonaws.com[-]

Example:
  [darkmagenta::b]  https://s3.us-east-1.amazonaws.com[-]

Use the AWS Console to find your bucket endpoint.
Available regions are listed in the dropdown.`
		endpointURL = "https://s3.amazonaws.com"
		region = "us-east-1"
		regions = awsRegions

	case "Other":
		urlRegionHelpText = `[::b]You selected a custom provider.[-]

Enter the endpoint URL and region manually.
Refer to your provider’s documentation for valid formats.`
		endpointURL = "https://your-storage-endpoint.com"
		region = "your-region"
	default:
		endpointURL = "https://s3.sv15.seagate.com"
		region = "us-east-1"
	}

	// Header and help text
	header := fmt.Sprintf(`[#6EBE49::b]Step 2: Enter Endpoint & Region for %s[-]
[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[white]
%s`, storageProvider, urlRegionHelpText)

	pageText := tview.NewTextView().
		SetText(header).
		SetTextAlign(tview.AlignCenter).
		SetWrap(true).
		SetDynamicColors(true)

	// URL input field
	urlInput := tview.NewInputField().
		SetLabel("🔗 Endpoint URL: ").
		SetText(endpointURL).
		SetFieldWidth(60).
		SetChangedFunc(func(text string) {
			endpointURL = text
		}).
		SetLabelColor(tcell.ColorYellow).
		SetFieldTextColor(tcell.ColorWhite).
		SetFieldBackgroundColor(tcell.ColorBlue)

	// Region input (dropdown or manual)
	if storageProvider != "Other" {
		regionInput = tview.NewDropDown().
			SetLabel("🌐 Region: ").
			SetOptions(regions, func(text string, index int) {
				region = text
			}).
			SetCurrentOption(0).
			SetLabelColor(tcell.ColorYellow).
			SetFieldTextColor(tcell.ColorWhite).
			SetFieldBackgroundColor(tcell.ColorBlue)
	} else {
		regionInput = tview.NewInputField().
			SetLabel("🌐 Region: ").
			SetText("Enter Region (e.g., us-east-1)").
			SetFieldWidth(30).
			SetLabelColor(tcell.ColorYellow).
			SetFieldTextColor(tcell.ColorWhite).
			SetFieldBackgroundColor(tcell.ColorBlue).
			SetChangedFunc(func(text string) {
				region = text
			})
	}

	// Navigation form
	form := tview.NewForm().
		AddFormItem(urlInput).
		AddFormItem(regionInput).
		AddButton("🏠 Home", func() {
			pages.SwitchToPage("home")
		}).
		AddButton("➡ Next", func() {
			if _, err := validateURL(endpointURL); err != nil {
				showModal(app, pages, "Invalid URL format.\nPlease try again.", func() {
					pages.SwitchToPage("page2")
				})
				return
			}
			pages.SwitchToPage("page3")
		}).
		AddButton("⬅ Back", func() {
			pages.SwitchToPage("page1")
		}).
		AddButton("📄 Preview", func() {
			if _, err := validateURL(endpointURL); err != nil {
				showModal(app, pages, "Invalid URL format.\nPlease try again.", func() {
					pages.SwitchToPage("page2")
				})
				return
			}
			previewPage = "page2"
			summaryPage := buildSummaryPage(app, pages)
			pages.AddPage("summaryPage", summaryPage, true, false)
			pages.SwitchToPage("summaryPage")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(menuButtonColor).
		SetButtonTextColor(menuButtonTextColor).
		SetButtonsAlign(tview.AlignCenter)

	// Final layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).
		AddItem(pageText, 14, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(form, 10, 0, true).
		AddItem(nil, 1, 0, false)

	return layout
}


// func buildCredentialsPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
// 	// Placeholder for credentials page
// 	pageText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`[green::b]Enter your Cloud Storage Credentials:[-]
		
// 	[yellow::b]Access Key:[-] This is your unique identifier for accessing your cloud storage.
// 	[yellow::b]Secret Key:[-] This is your secret password for accessing your cloud storage.
// 	[::i](Make sure to keep these credentials secure and do not share them with anyone.)[-]`)
	
// 	form := tview.NewForm()

// 	accessKeyField := tview.NewInputField().
// 		SetLabel("Access Key:").
// 		SetText(accessKey). // needs to be deleted after testing
// 		SetFieldWidth(24)

// 	secretKeyField := tview.NewInputField().
// 		SetLabel("Secret Key:").
// 		SetText(secretKey). // needs to be deleted after testing 
// 		SetFieldWidth(43).
// 		SetMaskCharacter('*')

// 	form.
// 		AddFormItem(accessKeyField).
// 		AddFormItem(secretKeyField).
// 		AddButton("Home", func() {
// 			pages.SwitchToPage("home")
// 		}).
// 		AddButton("Next", func() {
// 			// Validate credentials here: make sure that the keys are 20 characters long, only alphanumeric characters, no special characters
// 			accessKey := accessKeyField.GetText()
// 			secretKey := secretKeyField.GetText()
// 			// Convert to uppercase
// 			accessKey = strings.ToUpper(accessKey)
// 			// Check prefixes
// 			// if !strings.HasPrefix(accessKey, "AKIA") {
// 			// 	accessKey = "AKIA" + accessKey
// 			// }

// 			if len(accessKey) != 24 || len(secretKey) != 43 {
// 				showModal(app, pages, "Invalid credentials.\nPlease try again.", func() {
// 					pages.SwitchToPage("page3")
// 				})
// 				return
// 			}
// 			pages.SwitchToPage("page4")
// 		}).
// 		AddButton("Back", func() {
// 			pages.SwitchToPage("page2")
// 		}).
// 		AddButton("Preview", func() {
// 			summaryPage := buildSummaryPage(app, pages)
// 			pages.AddPage("summaryPage", summaryPage, true, false)
// 			pages.SwitchToPage("summaryPage") 
// 		}).
// 		AddButton("Quit", func() {
// 			app.Stop()
// 		})

// 	// form.SetBorder(true).SetTitle("[ Credentials ]").SetTitleAlign(tview.AlignLeft)
// 	form.SetButtonBackgroundColor(menuButtonColor).SetButtonTextColor(menuButtonTextColor)

// 	layout := tview.NewFlex().
// 		SetDirection(tview.FlexRow).
// 		AddItem(nil, 1, 1, false).           	// Top padding
// 		AddItem(pageText, 5, 1, false).      	// Main content
// 		AddItem(form, 8, 1, false).           	// Form for credentials
// 		AddItem(nil, 1, 1, false)             	// Bottom padding

// 	return layout
// }


// func buildCredentialsPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
// 	// Instructional text with consistent style
// 	pageText := tview.NewTextView().
// 		SetTextAlign(tview.AlignCenter).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`[#6EBE49::b]Step 3: Enter Your Cloud Storage Credentials[-]
// [#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// [white]
// [#FFD700::b]Access Key:[-] This is your unique identifier for accessing your cloud storage.
// [#FFD700::b]Secret Key:[-] This is your secret password for accessing your cloud storage.

// [::i]Please keep these credentials secure and do not share them with anyone.[-]`)

// 	// Access key input field
// 	accessKeyField := tview.NewInputField().
// 		SetLabel("🔑 Access Key: ").
// 		SetText(accessKey). // For testing – remove in production
// 		SetFieldWidth(24).
// 		SetLabelColor(tcell.ColorYellow).
// 		SetFieldTextColor(tcell.ColorWhite).
// 		SetFieldBackgroundColor(tcell.ColorBlue)

// 	// Secret key input field
// 	secretKeyField := tview.NewInputField().
// 		SetLabel("🔑 Secret Key: ").
// 		SetText(secretKey). // For testing – remove in production
// 		SetFieldWidth(43).
// 		SetMaskCharacter('*').
// 		SetLabelColor(tcell.ColorYellow).
// 		SetFieldTextColor(tcell.ColorWhite).
// 		SetFieldBackgroundColor(tcell.ColorBlue)

// 	// Credential form
// 	form := tview.NewForm().
// 		AddFormItem(accessKeyField).
// 		AddFormItem(secretKeyField).
// 		AddButton("🏠 Home", func() {
// 			pages.SwitchToPage("home")
// 		}).
// 		AddButton("➡ Next", func() {
// 			accessKey := strings.ToUpper(accessKeyField.GetText())
// 			secretKey := secretKeyField.GetText()

// 			if len(accessKey) != 24 || len(secretKey) != 43 {
// 				showModal(app, pages, "Invalid credentials.\nPlease try again.", func() {
// 					pages.SwitchToPage("page3")
// 				})
// 				return
// 			}
// 			// Dry run 
// 			pages.SwitchToPage("page4")
// 		}).
// 		AddButton("⬅ Back", func() {
// 			pages.SwitchToPage("page2")
// 		}).
// 		AddButton("📄 Preview", func() {
// 			summaryPage := buildSummaryPage(app, pages)
// 			pages.AddPage("summaryPage", summaryPage, true, false)
// 			pages.SwitchToPage("summaryPage")
// 		}).
// 		AddButton("❌ Quit", func() {
// 			app.Stop()
// 		}).
// 		SetButtonBackgroundColor(menuButtonColor).
// 		SetButtonTextColor(menuButtonTextColor).
// 		SetButtonsAlign(tview.AlignCenter)

// 	// Final layout
// 	layout := tview.NewFlex().
// 		SetDirection(tview.FlexRow).
// 		AddItem(nil, 1, 0, false).            // Top padding
// 		AddItem(pageText, 9, 0, false).       // Instructional text
// 		AddItem(nil, 1, 0, false).
// 		AddItem(form, 9, 0, true).            // Credential input form
// 		AddItem(nil, 1, 0, false)             // Bottom padding

// 	return layout
// }


func buildCredentialsPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	// Instructional text with consistent style
	pageText := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[#6EBE49::b]Step 3: Enter Your Cloud Storage Credentials[-]
[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[white]
[#FFD700::b]Access Key:[-] This is your unique identifier for accessing your cloud storage.
[#FFD700::b]Secret Key:[-] This is your secret password for accessing your cloud storage.

[::i]Please keep these credentials secure and do not share them with anyone.[-]`)

	// Access key input field
	accessKeyField := tview.NewInputField().
		SetLabel("🔑 Access Key: ").
		SetText(accessKey). // For testing – remove in production
		SetFieldWidth(24).
		SetLabelColor(tcell.ColorYellow).
		SetFieldTextColor(tcell.ColorWhite).
		SetFieldBackgroundColor(tcell.ColorBlue)

	// Secret key input field
	secretKeyField := tview.NewInputField().
		SetLabel("🔑 Secret Key: ").
		SetText(secretKey). // For testing – remove in production
		SetFieldWidth(43).
		SetMaskCharacter('*').
		SetLabelColor(tcell.ColorYellow).
		SetFieldTextColor(tcell.ColorWhite).
		SetFieldBackgroundColor(tcell.ColorBlue)

	// Credential form
	form := tview.NewForm().
		AddFormItem(accessKeyField).
		AddFormItem(secretKeyField).
		AddButton("🏠 Home", func() {
			pages.SwitchToPage("home")
		}).
		AddButton("➡ Next", func() {
			accessKey := strings.ToUpper(accessKeyField.GetText())
			secretKey := secretKeyField.GetText()

			if len(accessKey) != 24 || len(secretKey) != 43 {
				showModal(app, pages, "Invalid credentials.\nPlease try again.", func() {
					pages.SwitchToPage("page3")
				})
				return
			}

			// Step 1: Write a temp config.yaml with provided credentials
		// 	tempConfig := `
		// components: ["s3storage"]
		// s3storage:
		// access_key: "` + accessKey + `"
		// secret_key: "` + secretKey + `"
		// region: "us-east-1"
		// `
			// tmpFile := "config-tui-temp.yaml"
			// err := os.WriteFile(tmpFile, []byte(tempConfig), 0600)
			// if err != nil {
			// 	showModal(app, pages, "Failed to write config file:\n"+err.Error(), nil)
			// 	return
			// }

			tmpFile := "config-tui-temp.yaml"
			options.ConfigFile = tmpFile

			// Step 2: Parse the config
			err := parseConfig()
			if err != nil {
				showModal(app, pages, "Failed to parse config:\n"+err.Error(), nil)
				return
			}

			err = config.Unmarshal(&options)
			if err != nil {
				showModal(app, pages, "Failed to unmarshal config:\n"+err.Error(), nil)
				return
			}

			// Step 3: Try to fetch container/bucket list
			// var containerList []string
			if slices.Contains(options.Components, "azstorage") {
				containerList, err = getContainerListAzure()
			} else if slices.Contains(options.Components, "s3storage") {
				containerList, err = getBucketListS3()
			} else {
				err = fmt.Errorf("unsupported storage backend")
			}

			if err != nil {
				showModal(app, pages, "Failed to connect:\n"+err.Error(), nil)
				return
			}

			// Step 4: Pass containerList to page4 (next page)
			page4 := buildContainerSelectPage(app, pages)
			pages.AddPage("page4", page4, true, false)
			pages.SwitchToPage("page4")
		}).
		AddButton("⬅ Back", func() {
			pages.SwitchToPage("page2")
		}).
		AddButton("📄 Preview", func() {
			summaryPage := buildSummaryPage(app, pages)
			pages.AddPage("summaryPage", summaryPage, true, false)
			pages.SwitchToPage("summaryPage")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(menuButtonColor).
		SetButtonTextColor(menuButtonTextColor).
		SetButtonsAlign(tview.AlignCenter)

	// Final layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).            // Top padding
		AddItem(pageText, 9, 0, false).       // Instructional text
		AddItem(nil, 1, 0, false).
		AddItem(form, 9, 0, true).            // Credential input form
		AddItem(nil, 1, 0, false)             // Bottom padding

	return layout
}

// func buildBucketNamePage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	
// 	pageText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`[green::b]Select your Bucket/Container Name:[-]

// 	The bucket/container names available for your cloud storage provider are 
// 	listed below in the dropdown. The available bucket/container names are based 
// 	on the credentials you entered in the previous step.`)
	
// 	bucketNameField := tview.NewInputField().
// 		SetLabel("Bucket/Container Name:").
// 		SetText("my-bucket").
// 		SetFieldWidth(30)

// 	form := tview.NewForm().
// 		AddFormItem(bucketNameField).
// 		AddButton("Home", func() {
// 			pages.SwitchToPage("home")
// 		}).
// 		AddButton("Next", func() {
// 			bucketName = bucketNameField.GetText()
// 			if bucketName == "" {
// 				showModal(app, pages, "Bucket/container name cannot be empty.\nPlease try again.", func() {
// 					pages.SwitchToPage("page4")
// 				})
// 				return
// 			}
// 			// Proceed to the next step or page
// 			pages.SwitchToPage("page5")
// 		}).
// 		AddButton("Back", func() {
// 			pages.SwitchToPage("page3")
// 		}).
// 		AddButton("Preview", func() {
// 			summaryPage := buildSummaryPage(app, pages)
// 			pages.AddPage("summaryPage", summaryPage, true, false)
// 			pages.SwitchToPage("summaryPage") // Switch to Page 3
// 		}).
// 		AddButton("Quit", func() {
// 			app.Stop()
// 		})

// 	// form.SetBorder(true).SetTitle("[ Bucket/Container Name ]").SetTitleAlign(tview.AlignLeft)
// 	form.SetButtonBackgroundColor(menuButtonColor).SetButtonTextColor(menuButtonTextColor)

// 	layout := tview.NewFlex().
// 		SetDirection(tview.FlexRow).
// 		AddItem(nil, 1, 1, false).           	// Top padding
// 		AddItem(pageText, 5, 1, false).      	// Main content
// 		AddItem(form, 8, 1, false).           	// Form for bucket
// 		AddItem(nil, 1, 1, false)             	// Bottom padding

// 	return layout
// }


func buildContainerSelectPage(app *tview.Application, pages *tview.Pages) tview.Primitive {

	pageText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[#6EBE49::b]Step 4: Select Your Bucket or Container Name[-]
[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[white]
Enter the name of your storage bucket or container. These should be accessible 
based on the credentials you entered in the previous step.`)

	// Bucket name input
	// bucketNameField := tview.NewInputField().
	// 	SetLabel("🪣 Bucket/Container Name: ").
	// 	SetText("my-bucket").
	// 	SetFieldWidth(30).
	// 	SetLabelColor(tcell.ColorYellow).
	// 	SetFieldTextColor(tcell.ColorWhite).
	// 	SetFieldBackgroundColor(tcell.ColorBlue)
	containerNameDropdown := tview.NewDropDown().
		SetLabel("🪣 Bucket/Container Name: ").
		SetOptions(containerList, func(text string, index int) {
			bucketName = text
		}).
		SetCurrentOption(0).
		SetLabelColor(tcell.ColorYellow).
		SetFieldTextColor(tcell.ColorWhite).
		SetFieldBackgroundColor(tcell.ColorBlue).
		SetFieldWidth(30)

	// Form with navigation
	form := tview.NewForm().
		// AddFormItem(bucketNameField).
		AddFormItem(containerNameDropdown).
		AddButton("🏠 Home", func() {
			pages.SwitchToPage("home")
		}).
		AddButton("➡ Next", func() {
			// bucketName = containerName.GetText()
			// if strings.TrimSpace(bucketName) == "" {
			// 	showModal(app, pages, "Bucket/container name cannot be empty.\nPlease try again.", func() {
			// 		pages.SwitchToPage("page4")
			// 	})
			// 	return
			// }
			pages.SwitchToPage("page5")
		}).
		AddButton("⬅ Back", func() {
			pages.SwitchToPage("page3")
		}).
		AddButton("📄 Preview", func() {
			summaryPage := buildSummaryPage(app, pages)
			pages.AddPage("summaryPage", summaryPage, true, false)
			pages.SwitchToPage("summaryPage")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(menuButtonColor).
		SetButtonTextColor(menuButtonTextColor).
		SetButtonsAlign(tview.AlignCenter)

	// Final layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).            // Top padding
		AddItem(pageText, 7, 0, false).       // Instructional text
		AddItem(nil, 1, 0, false).
		AddItem(form, 9, 0, true).            // Input form
		AddItem(nil, 1, 0, false)             // Bottom padding

	return layout
}

// func buildCachingPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
// 	// Placeholder for caching page
// 	pageText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`[green::b]Configure Caching Settings:[-]

// 	To help Cloudfuse work best for you, let's determine how you'd like to use your local storage.`)


// 	localCacheText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`Do you have available local storage that you'd like 
// Cloudfuse to use for improved performance and reliability?`)

// 	cacheToDisk := tview.NewDropDown().
// 		SetLabel("Cache to local disk? ").
// 		SetOptions([]string{" Yes ", " No "}, func(text string, index int) {
// 			if text == "Yes" {
// 				// Enable disk caching settings
// 			} else {
// 				// Disable disk caching settings
// 			}
// 		}).
// 		SetCurrentOption(0)

// 	cacheLocationText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`If you selected "Yes" above, please enter the location of the cache directory.
// 		For example, /var/cache/s3storage or /tmp/s3cache.`)

// 	cacheLocationField := tview.NewInputField().
// 		SetLabel("Cache Location:").
// 		SetText("/var/cache/s3storage").
// 		SetFieldWidth(30).
// 		SetChangedFunc(func(text string) {
// 			// Validate cache location input
// 			if text == "" {
// 				showModal(app, pages, "Cache location cannot be empty.\nPlease try again.", func() {
// 					pages.SwitchToPage("page5")
// 				})
// 				return
// 			}
// 			// Update cache location if needed
// 			cacheLocation = text
			
// 		})

// 	cacheSizeText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`We've detected [X GB] available at this location. 
// 		By default, Cloudfuse will use up to [80% of X GB] for its cache. 
// 		Would you like to specify a different cache size (in GB)?`)

// 	// Cache size input field. How much space (at most) do they want the cache to take up (default is 80% of available space on the drive which contains the directory they entered above)
// 	cacheSizeField := tview.NewInputField().
// 		SetLabel("Cache Size:").
// 		SetText("80"). // Default to 80%
// 		SetFieldWidth(10).
// 		SetChangedFunc(func(text string) {
// 			// Validate cache size input
// 			if size, err := strconv.Atoi(text); err != nil || size < 1 || size > 100 {
// 				showModal(app, pages, "Cache size must be between 1 and 100.\nPlease try again.", func() {
// 					pages.SwitchToPage("page5")
// 				})
// 				return
// 			}
// 			cacheSize = text // Update cache size
// 		})	

// 	cacheRetentionText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`Do you need cached data to be automatically removed from 
// 		local storage after a certain amount of time since its last access?`)


// 	cacheRetention := tview.NewCheckbox().
// 		SetLabel("Enable Cache Retention?").
// 		SetChecked(false).
// 		SetChangedFunc(func(checked bool) {
// 			if checked {
// 				// Enable cache retention settings
// 			} else {
// 				// Disable cache retention settings
// 			}
// 		})

// 	cacheRetentionDurationText := tview.NewTextView().
// 		SetTextAlign(tview.AlignLeft).
// 		SetWrap(true).
// 		SetDynamicColors(true).
// 		SetText(`If you selected "Yes" above, please enter the duration and 
// 		select the unit for cache retention from the dropdown`)

// 	cacheRetentionDurationUnit := tview.NewForm().
// 		AddInputField("Cache Retention Duration:", "30", 10, nil, func(text string) {
// 			// Validate cache retention duration input
// 			cacheRetentionDuration = text
// 		}).
// 		AddDropDown("Unit:", []string{"Seconds", "Minutes", "Hours", "Days"}, 0, func(option string, index int) {
// 			retentionUnit = option
// 		})


// 	menuButtons := tview.NewForm().
// 		AddButton("Home", func() {
// 			pages.SwitchToPage("home")
// 		}).
// 		AddButton("Finish", func() {
// 			app.Stop()
// 		}).
// 		AddButton("Back", func() {
// 			pages.SwitchToPage("page4")
// 		}).
// 		AddButton("Preview", func() {
// 			summaryPage := buildSummaryPage(app, pages)
// 			pages.AddPage("summaryPage", summaryPage, true, false)
// 			pages.SwitchToPage("summaryPage") 
// 		}).
// 		AddButton("Quit", func() {
// 			app.Stop()
// 		})

// 	// form.SetBorder(true).SetTitle("[ Caching Settings ]").SetTitleAlign(tview.AlignLeft)
// 	menuButtons.SetButtonBackgroundColor(menuButtonColor).SetButtonTextColor(menuButtonTextColor)

// 	layout := tview.NewFlex().
// 		SetDirection(tview.FlexRow).
// 		AddItem(nil, 1, 1, false).           	// Top padding
// 		AddItem(pageText, 5, 1, false).      	// Main content
// 		AddItem(localCacheText, 3, 1, false). 	// Local
// 		AddItem(cacheToDisk, 3, 1, false).    	// Cache to disk dropdown
// 		AddItem(cacheLocationText, 3, 1, false). // Cache location text
// 		AddItem(cacheLocationField, 3, 1, false).  	// Cache location
// 		AddItem(cacheSizeText, 3, 1, false).  	// Cache
// 		AddItem(cacheSizeField, 3, 1, false). 	// Cache size input
// 		AddItem(cacheRetentionText, 3, 1, false). // Cache retention text
// 		AddItem(cacheRetention, 3, 1, false).  	// Cache retention
// 		AddItem(cacheRetentionDurationText, 3, 1, false). // Cache retention
// 		AddItem(cacheRetentionDurationUnit, 5, 1, false).  	// Cache retention
// 		AddItem(menuButtons, 3, 1, false).     	// Menu buttons
// 		AddItem(nil, 1, 1, false)             	// Bottom padding

// 	return layout
// }


func buildCachingPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	pageText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[#6EBE49::b]Step 5: Configure Caching Settings[-]
[#FFD700]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[white]
To optimize performance and reliability, you can allow CloudFuse to cache
data locally on your disk. You can customize where, how much, and for how long
this cache is used.`)

	localCacheText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[::b]💾 Do you want to enable local caching?[-:-]
Enable this if you have enough local storage available. Cached data improves
performance and resilience when the cloud is temporarily unavailable.`)

	cacheToDisk := tview.NewDropDown().
		SetLabel("📁 Cache to Local Disk: ").
		SetOptions([]string{" Yes ", " No "}, func(text string, index int) {
			// optional logic could be added to enable/disable below fields dynamically
		}).
		SetCurrentOption(0).
		SetLabelColor(tcell.ColorYellow).
		SetFieldBackgroundColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorWhite)

	cacheLocationText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[::b]📂 Cache Directory Location:[-:-]
Enter the absolute path to a directory where CloudFuse can store cached files.
Example: [blue]/var/cache/s3storage[-] or [blue]/tmp/cloudcache[-]`)

	cacheLocationField := tview.NewInputField().
		SetLabel("📍 Cache Location: ").
		SetText("/var/cache/s3storage").
		SetFieldWidth(40).
		SetLabelColor(tcell.ColorYellow).
		SetFieldBackgroundColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorWhite).
		SetChangedFunc(func(text string) {
			if strings.TrimSpace(text) == "" {
				showModal(app, pages, "Cache location cannot be empty.\nPlease try again.", func() {
					pages.SwitchToPage("page5")
				})
				return
			}
			cacheLocation = text
		})

	cacheSizeText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[::b]🧠 Cache Size (in GB):[-:-]
Specify how much disk space to allow CloudFuse for cache storage.
Recommended default is 80%% of available space on the chosen drive.`)

	cacheSizeField := tview.NewInputField().
		SetLabel("📦 Cache Size (GB): ").
		SetText("80").
		SetFieldWidth(10).
		SetLabelColor(tcell.ColorYellow).
		SetFieldBackgroundColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorWhite).
		SetChangedFunc(func(text string) {
			if size, err := strconv.Atoi(text); err != nil || size < 1 || size > 100 {
				showModal(app, pages, "Cache size must be between 1 and 100.\nPlease try again.", func() {
					pages.SwitchToPage("page5")
				})
				return
			}
			cacheSize = text
		})

	cacheRetentionText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[::b]🕒 Cache Retention Settings:[-:-]
You can optionally have cached files auto-deleted if they haven’t been
accessed in a while.`)

	cacheRetention := tview.NewCheckbox().
		SetLabel("🧹 Enable Cache Retention: ").
		SetChecked(false).
		SetLabelColor(tcell.ColorYellow).
		SetChangedFunc(func(checked bool) {
			// Logic could enable/disable retention duration input dynamically
		})

	cacheRetentionDurationText := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(`[::b]⏳ Retention Duration:[-:-]
If retention is enabled, enter the duration and unit below.
For example: 30 [blue]Days[-] or 12 [blue]Hours[-].`)

	cacheRetentionDurationUnit := tview.NewForm().
		AddInputField("⏱ Duration:", "30", 10, nil, func(text string) {
			cacheRetentionDuration = text
		}).
		AddDropDown("🕰 Unit:", []string{"Seconds", "Minutes", "Hours", "Days"}, 0, func(option string, index int) {
			cacheRetentionUnit = option
		}).
		SetLabelColor(tcell.ColorYellow).
		SetFieldBackgroundColor(tcell.ColorBlue).
		SetFieldTextColor(tcell.ColorWhite)

	// Navigation buttons
	menuButtons := tview.NewForm().
		AddButton("🏠 Home", func() {
			pages.SwitchToPage("home")
		}).
		AddButton("✅ Finish", func() {
			app.Stop()
		}).
		AddButton("⬅ Back", func() {
			pages.SwitchToPage("page4")
		}).
		AddButton("📄 Preview", func() {
			summaryPage := buildSummaryPage(app, pages)
			pages.AddPage("summaryPage", summaryPage, true, false)
			pages.SwitchToPage("summaryPage")
		}).
		AddButton("❌ Quit", func() {
			app.Stop()
		}).
		SetButtonBackgroundColor(menuButtonColor).
		SetButtonTextColor(menuButtonTextColor).
		SetButtonsAlign(tview.AlignCenter)

	// Layout
	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 1, 0, false).                          // Top padding
		AddItem(pageText, 6, 0, false).
		AddItem(localCacheText, 3, 0, false).
		AddItem(cacheToDisk, 2, 0, false).
		AddItem(cacheLocationText, 3, 0, false).
		AddItem(cacheLocationField, 2, 0, false).
		AddItem(cacheSizeText, 3, 0, false).
		AddItem(cacheSizeField, 2, 0, false).
		AddItem(cacheRetentionText, 3, 0, false).
		AddItem(cacheRetention, 2, 0, false).
		AddItem(cacheRetentionDurationText, 2, 0, false).
		AddItem(cacheRetentionDurationUnit, 4, 0, false).
		AddItem(menuButtons, 3, 0, false).
		AddItem(nil, 1, 0, false)                          // Bottom padding

	return layout
}


// func buildSummaryPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
// 	// Rebuild the modal each time, using the updated values
	
// 	summaryText := fmt.Sprintf(
// 		"[yellow::b]Summary Configuration for %s:\n\n"+
// 		"Storage Provider: %s\n"+
// 		"Endpoint URL: %s\n"+
// 		"Region: %s\n"+
// 		"Bucket/Container Name: %s\n"+
// 		"Cache Mode: %s\n"+
// 		"Cache Size: %s GB\n"+
// 		"Cache Retention: %s %s\n",
// 		storageProvider, storageProvider, urlText, regionText, bucketName,
// 		cacheMode, cacheSize, retentionUnit, cacheRetentionDuration,
// 	)

// 	modal := tview.NewModal().
// 		SetText(summaryText).
// 		AddButtons([]string{"Return"}).
// 		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
// 			pages.SwitchToPage(previewPage)
// 		})

// 	return modal
// }

func buildSummaryPage(app *tview.Application, pages *tview.Pages) tview.Primitive {
	summary := fmt.Sprintf(
		"[green::b]\t\tCloudFuse Summary Configuration:[-]\n\n"+
			"Storage Provider: [yellow::b]%s[-]\n"+
			"    Endpoint URL: [yellow::b]%s[-]\n"+
			"          Region: [yellow::b]%s[-]\n"+
			"  Container Name: [yellow::b]%s[-]\n"+
			"      Cache Mode: [yellow::b]%s[-]\n"+
			"      Cache Size: [yellow::b]%s GB[-]\n"+
			" Cache Retention: [yellow::b]%s %s[-]\n",
		storageProvider, endpointURL, region, bucketName,
		cacheMode, cacheSize, cacheRetentionDuration, cacheRetentionUnit,
	)

	textView := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetWrap(true).
		SetDynamicColors(true).
		SetText(summary).
		SetScrollable(true).
		SetChangedFunc(func() {
			app.Draw()
		})

	buttons := tview.NewFlex().SetDirection(tview.FlexColumn)

	returnButton := tview.NewButton("Return").
		SetSelectedFunc(func() {
			pages.SwitchToPage(previewPage)
		})

	buttons.AddItem(nil, 0, 1, false)    // Spacer
	buttons.AddItem(returnButton, 12, 1, true)
	buttons.AddItem(nil, 0, 1, false)    // Spacer

	frame := tview.NewFrame(textView).
		SetBorders(1, 1, 1, 1, 2, 2)
		// AddText("Summary", true, tview.AlignCenter, tcell.ColorYellow)

	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(frame, 16, 1, false).
		AddItem(buttons, 3, 0, true)

	leftAlignedModal := tview.NewFlex().
		AddItem(modal, 60, 0, true).  // fixed width modal on the left
		AddItem(nil, 0, 1, false)     // spacer on the right

	return leftAlignedModal
}


// Helper to show modals (e.g., for errors)
func showModal(app *tview.Application, pages *tview.Pages, message string, onClose func()) {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("modal")
			onClose()
		})
	pages.AddPage("modal", modal, false, true)
}


// Helper function to normalize and validate the URL
func validateURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "https://" + rawURL
	}

	if _, err := url.ParseRequestURI(rawURL); err != nil {
		return "", fmt.Errorf("invalid URL format")
	}

	return rawURL, nil
}


// Function to create YAML configuration file from all data collected from the TUI
func createYAMLConfig() {
	
	config := Config{
		Logging: LoggingConfig{
			Type:  "syslog",
			Level: "log_warning",
		},

		Components: []string{"libfuse", cacheMode, "attr_cache", storageProtocol},

		Libfuse: LibfuseConfig{
			AttributeExpirationSec:     120,
			EntryExpirationSec:         120,
			NegativeEntryExpirationSec: 240,
			NetworkShare:               true,
		},
		
		// Stream: StreamConfig{
		// 	BlockSizeMB:   8,
		// 	BlocksPerFile: 3,
		// 	CacheSizeMB:   1024,
		// },

		AttrCache: AttrCacheConfig{
			TimeoutSec: 7200,
		},	
	}

	switch cacheMode {
		case "file_cache":
			config.FileCache = FileCacheConfig{
				Path:           "Path/to/cache/dir",
				TimeOutSec:     64000000,
				CleanUpOnStart: true,
				IgnoreSync:     true,
			}
		case "block_cache":
			config.BlockCache = BlockCacheConfig{
				BlockSizeMB:  8,
				MemorySizeMB: 1024,
				Prefetch:     2,
				Parallelism:  4,
			}
		default: // "stream" or any unrecognized mode defaults to stream
			config.Stream = StreamConfig{
				BlockSizeMB:   8,
				BlocksPerFile: 3,
				CacheSizeMB:   1024,
			}
	}


	if storageProtocol == "s3storage" {
		config.S3Storage = S3StorageConfig{
			BucketName:      bucketName, // This should be set from the bucket
			KeyID:           accessKey, // This should be set from the access key input
			SecretKey:       secretKey, // This should be set from the secret key input
			Endpoint:        endpointURL, // This should be set from the URL input
			Region:          region, // This should be set from the region input
			EnableDirMarker: true, // Default to true, can be changed in the TUI
		}
	} else {
		config.AzStorage = &AzureStorageConfig{
			Type:        "block",
			AccountName: accountName, // This should be set from the account name input
			AccountKey:  secretKey, // This should be set from the account key input
			Endpoint:    endpointURL, // This should be set from the URL input
			Mode:        "key", // Default mode, can be changed in the TUI
			Container:   bucketName, // This should be set from the container name input
		}
	}	

    // Marshal the struct to YAML (returns []byte and error)
    yamlData, err := yaml.Marshal(&config)
    if err != nil {
		fmt.Printf("Failed to marshal YAML: %v", err)
    }

    // Write the YAML to a file
    if err := os.WriteFile("config.yaml", yamlData, 0644); err != nil {
        fmt.Printf("Failed to write YAML to file: %v", err)
    }

    fmt.Printf("YAML config written to config.yaml\n")

}

var (
	azureRegions = []string{
		"us-east", "us-west", "us-central", "us-south",
		"eu-west", "eu-central", "eu-south", "eu-north",
		"asia-east", "asia-west", "asia-south", "asia-central",
		"au-east", "au-west", "au-central", "au-south",
		"sa-east", "sa-west", "sa-central", "sa-south",
		"africa-north", "africa-south", "africa-west", "africa-east",
		"canada-east", "canada-west", "canada-central", "canada-south",
		"middle-east-north", "middle-east-south", "middle-east-central",
		"japan-east", "japan-west", "japan-central", "japan-south" }

	awsRegions = []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"af-south-1", "ap-east-1", "ap-south-1", "ap-south-2",
		"ap-southeast-1", "ap-southeast-2", "ap-southeast-3",
		"ap-southeast-4", "ap-southeast-5", "ap-southeast-7",
		"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ca-central-1", "ca-west-1", "eu-central-1",
		"eu-west-1", "eu-west-2", "eu-west-3",
		"eu-south-1", "eu-south-2", "eu-north-1",
		"eu-central-2", "il-central-1", "mx-central-1",
		"me-south-1", "me-central-1", "sa-east-1",
	}

	lyvecloudRegions = []string{
		"us-east-1", "us-west-1", "us-central-1", "eu-west-1",
	}

)