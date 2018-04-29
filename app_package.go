// Copyright (C) 2013-2018, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
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
          "Schema": "{\n	\"title\": \"sampleEntry Schema\",\n	\"type\": \"object\",\n	\"properties\": {\n		\"content\": {\n			\"type\": \"string\"\n		},\n		\"timestamp\": {\n			\"type\": \"integer\"\n		}\n	},\n    \"required\": [\"content\", \"timestamp\"]\n}"
        }
      ],

      # Zome Functions
      # Functions which can be called in your Zome's API.
      "Functions": [
        {
          "Name": "sampleEntryCreate", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public" # Level to which is this function exposed.
        },
        {
          "Name": "sampleEntryRead", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public" # Level to which is this function exposed.
        },
        {
          "Name": "doSampleAction", # The name of this function.
          "CallingType": "json", # Data format for parameters passed to this function.
          "Exposure": "public" # Level to which is this function exposed.
        }
      ],

      # Zome Source Code
      # The logic that will control Zome behavior
      "Code": "/*******************************************************************************\n * Utility functions\n ******************************************************************************/\n\n/**\n * Is this a valid entry type?\n *\n * @param {any} entryType The data to validate as an expected entryType.\n * @return {boolean} true if the passed argument is a valid entryType.\n */\nfunction isValidEntryType (entryType) {\n  // Add additonal entry types here as they are added to dna.json.\n  return [\"sampleEntry\"].includes(entryType);\n}\n\n/**\n * Returns the creator of an entity, given an entity hash.\n *\n * @param  {string} hash The entity hash.\n * @return {string} The agent hash of the entity creator.\n */\nfunction getCreator (hash) {\n  return get(hash, { GetMask: HC.GetMask.Sources })[0];\n}\n\n/*******************************************************************************\n * Required callbacks\n ******************************************************************************/\n\n/**\n * System genesis callback: Can the app start?\n *\n * Executes just after the initial genesis entries are committed to your chain\n * (1st - DNA entry, 2nd Identity entry). Enables you specify any additional\n * operations you want performed when a node joins your holochain.\n *\n * @return {boolean} true if genesis is successful and so the app may start.\n *\n * @see https://developer.holochain.org/API#genesis\n */\nfunction genesis () {\n  return true;\n}\n\n/**\n * Validation callback: Can this entry be committed to a source chain?\n *\n * @param  {string} entryType Type of the entry as per DNA config for this zome.\n * @param  {string|object} entry Data with type as per DNA config for this zome.\n * @param  {Header-object} header Header object for this entry.\n * @param  {Package-object|null} pkg Package object for this entry, if exists.\n * @param  {string[]} sources Array of agent hashes involved in this commit.\n * @return {boolean} true if this entry may be committed to a source chain.\n *\n * @see https://developer.holochain.org/API#validateCommit_entryType_entry_header_package_sources\n * @see https://developer.holochain.org/Validation_Functions\n */\nfunction validateCommit (entryType, entry, header, pkg, sources) {\n  return isValidEntryType(entryType);\n}\n\n/**\n * Validation callback: Can this entry be committed to the DHT on any node?\n *\n * It is very likely that this validation routine should check the same data\n * integrity as validateCommit, but, as it happens during a different part of\n * the data life-cycle, it may require additional validation steps.\n *\n * This function will only get called on entry types with \"public\" sharing, as\n * they are the only types that get put to the DHT by the system.\n *\n * @param  {string} entryType Type of the entry as per DNA config for this zome.\n * @param  {string|object} entry Data with type as per DNA config for this zome.\n * @param  {Header-object} header Header object for this entry.\n * @param  {Package-object|null} pkg Package object for this entry, if exists.\n * @param  {string[]} sources Array of agent hashes involved in this commit.\n * @return {boolean} true if this entry may be committed to the DHT.\n *\n * @see https://developer.holochain.org/API#validatePut_entryType_entry_header_package_sources\n * @see https://developer.holochain.org/Validation_Functions\n */\nfunction validatePut (entryType, entry, header, pkg, sources) {\n  return validateCommit(entryType, entry, header, pkg, sources);\n}\n\n/**\n * Validation callback: Can this entry be modified?\n *\n * Validate that this entry can replace 'replaces' due to 'mod'.\n *\n * @param  {string} entryType Type of the entry as per DNA config for this zome.\n * @param  {string|object} entry Data with type as per DNA config for this zome.\n * @param  {Header-object} header Header object for this entry.\n * @param  {string} replaces The hash string of the entry being replaced.\n * @param  {Package-object|null} pkg Package object for this entry, if exists.\n * @param  {string[]} sources Array of agent hashes involved in this mod.\n * @return {boolean} true if this entry may replace 'replaces'.\n *\n * @see https://developer.holochain.org/API#validateMod_entryType_entry_header_replaces_package_sources\n * @see https://developer.holochain.org/Validation_Functions\n */\nfunction validateMod (entryType, entry, header, replaces, pkg, sources) {\n  return validateCommit(entryType, entry, header, pkg, sources)\n    // Only allow the creator of the entity to modify it.\n    && getCreator(header.EntryLink) === getCreator(replaces);\n}\n\n/**\n * Validation callback: Can this entry be deleted?\n *\n * @param  {string} entryType Name of the entry as per DNA config for this zome.\n * @param  {string} hash The hash of the entry to be deleted.\n * @param  {Package-object|null} pkg Package object for this entry, if exists.\n * @param  {string[]} sources Array of agent hashes involved in this delete.\n * @return {boolean} true if this entry can be deleted.\n *\n * @see https://developer.holochain.org/API#validateDel_entryType_hash_package_sources\n * @see https://developer.holochain.org/Validation_Functions\n */\nfunction validateDel (entryType, hash, pkg, sources) {\n  return isValidEntryType(entryType)\n    // Only allow the creator of the entity to delete it.\n    && getCreator(hash) === sources[0];\n}\n\n/**\n * Package callback: The package request for validateCommit() and valdiatePut().\n *\n * Both 'commit' and 'put' trigger 'validatePutPkg' as 'validateCommit' and\n * 'validatePut' must both have the same data.\n *\n * @param  {string} entryType Name of the entry as per DNA config for this zome.\n * @return {PkgReq-object|null}\n *   null if the data required is the Entry and Header.\n *   Otherwise a \"Package Request\" object, which specifies what data to be sent\n *   to the validating node.\n *\n * @see https://developer.holochain.org/API#validatePutPkg_entryType\n * @see https://developer.holochain.org/Validation_Packaging\n */\nfunction validatePutPkg (entryType) {\n  return null;\n}\n\n/**\n * Package callback: The package request for validateMod().\n *\n * @param  {string} entryType Name of the entry as per DNA config for this zome.\n * @return {PkgReq-object|null}\n *   null if the data required is the Entry and Header.\n *   Otherwise a \"Package Request\" object, which specifies what data to be sent\n *   to the validating node.\n *\n * @see https://developer.holochain.org/API#validateModPkg_entryType\n * @see https://developer.holochain.org/Validation_Packaging\n */\nfunction validateModPkg (entryType) {\n  return null;\n}\n\n/**\n * Package callback: The package request for validateDel().\n *\n * @param  {string} entryType Name of the entry as per DNA config for this zome.\n * @return {PkgReq-object|null}\n *   null if the data required is the Entry and Header.\n *   Otherwise a \"Package Request\" object, which specifies what data to be sent\n *   to the validating node.\n *\n * @see https://developer.holochain.org/API#validateDelPkg_entryType\n * @see https://developer.holochain.org/Validation_Packaging\n */\nfunction validateDelPkg (entryType) {\n  return null;\n}"
    }
  ]},
"TestSets":[{
  "Name":"sample",
  "TestSet":{"Tests":[{"Convey":"Example: (which fails because we haven't actually implemented a sampleEntryCreate function) We can create a new sampleEntry","Zome":"sampleZome","FnName": "sampleEntryCreate","Input": {"content": "this is the entry body","stamp":12345},"Output":"\"%h1%\"","Exposure":"public"}]}}
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
