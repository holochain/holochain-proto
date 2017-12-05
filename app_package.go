// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------
// AppPackage implements loading DNA and other app information from an AppPackage structure

package holochain

import (
	"io"
)

const (
	AppPackageVersion = "0.0.1"
)

type AppPackageUIFile struct {
	FileName string
	Data     string
	Encoding string
}

type AppPackageTests struct {
	Name    string
	TestSet TestSet
}

type AppPackageScenario struct {
	Name   string
	Roles  []AppPackageTests
	Config TestConfig
}

type AppPackage struct {
	Version   string
	Generator string
	DNA       DNA
	TestSets  []AppPackageTests
	UI        []AppPackageUIFile
	Scenarios []AppPackageScenario
}

// LoadAppPackage decodes DNA and other appPackage data from appPackage file (via an io.reader)
func LoadAppPackage(reader io.Reader, encodingFormat string) (appPackageP *AppPackage, err error) {
	var appPackage AppPackage
	err = Decode(reader, encodingFormat, &appPackage)
	if err != nil {
		return
	}
	appPackageP = &appPackage
	appPackage.DNA.PropertiesSchema = `{
	"title": "Properties Schema",
	"type": "object",
	"properties": {
		"description": {
			"type": "string"
		},
		"language": {
			"type": "string"
		}
	}
}
`
	return
}

const (
	BasicTemplateAppPackageFormat = "yml"
)

var BasicTemplateAppPackage string = `{
 # AppPackage Version
 # The app package schema version of this file.
"Version": "` + AppPackageVersion + `",
"Generator": "holochain",

"DNA": {
  # This is a holochain application package yaml definition. http://ceptr.org/projects/holochain

  # DNA File Version
  # Version indicator for changes to DNA
  "Version": 1,

  # DNA Unique ID
  # This ID differentiates your app from others. For example, to tell one Slack team from another with same code, so change it!
  "UUID": "00000000-0000-0000-0000-000000000000",

  # Application Name
  # What would you like to call your holochain app?
  "Name": "templateApp",

  # Requires Holochain Version
  # Version indicator for which minimal version of holochain is required by this DNA
  "RequiresVersion": ` + VersionStr + `,

  # Properties
  # Properties that you want available across all Zomes.
  "Properties": {

    # Application Description
    # Briefly describe your holochain app.
    "description": "provides an application template",

    # Language
    # The base (human) language of this holochain app.
    "language": "en"
  },

  # Properties Schema File
  # Describes the entries in the Properties section of your dna file.
  "PropertiesSchemaFile": "properties_schema.json",

  # DHT Settings
  # Configure the properties of your Distributed Hash Table (e.g. hash algorithm, neighborhood size, etc.).
  "DHTConfig": {
    "HashType": "sha2-256"
  },

  # Zomes
  # List the Zomes your application will support.
  "Zomes": [
    {

      # Zome Name
      # The name of this code module.
      "Name": "sampleZome",

      # Zome Description
      # What is the purpose of this module?
      "Description": "provide a sample zome",

      # Ribosome Type
      # What scripting language will you code in?
      "RibosomeType": "js",

      # Zome Entries
      # Data stored and tracked by your Zome.
      "Entries": [
        {
          "Name": "sampleEntry", # The name of this entry.
          "Required": true, # Is this entry required?
          "DataFormat": "json", # What type of data should this entry store?
          "Sharing": "public", # Should this entry be publicly accessible?
          "Schema": "{\n	\"title\": \"sampleEntry Schema\",\n	\"type\": \"object\",\n	\"properties\": {\n		\"content\": {\n			\"type\": \"string\"\n		},\n		\"timestamp\": {\n			\"type\": \"integer\"\n		}\n	},\n    \"required\": [\"body\", \"timestamp\"]\n}",
          "_": "cr"
        }
      ],

      # Zome Functions
      # Functions which can be called in your Zome's API.
      "Functions": [
        {
          "Name": "sampleEntryCreate", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public", # Level to which is this function exposed.
          "_": "c:sampleEntry"
        },
        {
          "Name": "sampleEntryRead", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public", # Level to which is this function exposed.
          "_": "r:sampleEntry"
        },
        {
          "Name": "doSampleAction", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public" # Level to which is this function exposed.
        }
      ],

      # Zome Source Code
      # The logic that will control Zome behavior
      "Code": "'use strict';\n\n// -----------------------------------------------------------------\n//  This stub Zome code file was auto-generated\n// -----------------------------------------------------------------\n\n/**\n * Called only when your source chain is generated\n * @return {boolean} success\n */\nfunction genesis() {\n  // any genesis code here\n  return true;\n}\n\n// -----------------------------------------------------------------\n//  validation functions for every DHT entry change\n// -----------------------------------------------------------------\n\n/**\n * Called to validate any changes to the DHT\n * @param {string} entryName - the name of entry being modified\n * @param {*} entry - the entry data to be set\n * @param {?} header - ?\n * @param {?} pkg - ?\n * @param {?} sources - ?\n * @return {boolean} is valid?\n */\nfunction validateCommit (entryName, entry, header, pkg, sources) {\n  switch (entryName) {\n    case \"sampleEntry\":\n      // validation code here\n      return false;\n    default:\n      // invalid entry name!!\n      return false;\n  }\n}\n\n/**\n * Called to validate any changes to the DHT\n * @param {string} entryName - the name of entry being modified\n * @param {*}entry - the entry data to be set\n * @param {?} header - ?\n * @param {?} pkg - ?\n * @param {?} sources - ?\n * @return {boolean} is valid?\n */\nfunction validatePut (entryName, entry, header, pkg, sources) {\n  switch (entryName) {\n    case \"sampleEntry\":\n      // validation code here\n      return false;\ndefault:\n      // invalid entry name!!\n      return false;\n  }\n}\n\n/**\n * Called to validate any changes to the DHT\n * @param {string} entryName - the name of entry being modified\n * @param {*} entry- the entry data to be set\n * @param {?} header - ?\n * @param {*} replaces - the old entry data\n * @param {?} pkg - ?\n * @param {?} sources - ?\n * @return {boolean} is valid?\n */\nfunction validateMod (entryName, entry, header, replaces, pkg, sources) {\n  switch (entryName) {\n    case \"sampleEntry\":\n      // validation code here\n      return false;\n    default:\n      // invalid entry name!!\n      return false;\n  }\n}\n\n/**\n * Called to validate any changes to the DHT\n * @param {string} entryName - the name of entry being modified\n * @param {string} hash - the hash of the entry to remove\n * @param {?} pkg - ?\n * @param {?} sources - ?\n * @return {boolean} is valid?\n */\nfunction validateDel (entryName,hash, pkg, sources) {\n  switch (entryName) {\n    case \"sampleEntry\":\n      // validation code here\nreturn false;\n    default:\n      // invalid entry name!!\n      return false;\n  }\n}\n\n/**\n * Called to get the data needed to validate\n * @param {string} entryName - the name of entry to validate\n * @return {*} the data required for validation\n */\nfunction validatePutPkg (entryName) {\n  return null;\n}\n\n/**\n * Called to get the data needed to validate\n * @param {string} entryName - the name of entry to validate\n * @return {*} the data required for validation\n */\nfunction validateModPkg (entryName) {\n  return null;\n}\n\n/**\n * Called to get the data needed to validate\n * @param {string} entryName - the name of entry to validate\n * @return {*} the data required for validation\n */\nfunction validateDelPkg (entryName) {\n  return null;\n}"
    }
  ]},
"TestSets":[{
  "Name":"sample",
  "TestSet":{"Tests":[{"Convey":"We can create a new sampleEntry","FnName": "sampleEntryCreate","Input": {"body": "this is the entry body","stamp":12345},"Output":"\"%h1%\"","Exposure":"public"}]}}
   ],
"UI":[
{"FileName":"index.html",
 "Data":"<html><body>Your UI here!</body></html>"
},
{"FileName":"hc.js",
 "Data":"function yourApp(){alert('your UI code here!')}"
}],
"Scenarios":[
        {"Name":"sampleScenario",
         "Roles":[
             {"Name":"listener",
              "TestSet":{"Tests":[
                  {"Convey":"add listener test here"}]}},
             {"Name":"speaker",
              "TestSet":{"Tests":[
                  {"Convey":"add speaker test here"}]}}],
         "Config":{"Duration":5,"GossipInterval":100}}]
}
`
