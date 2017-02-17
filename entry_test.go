package holochain

import (
	"encoding/json"
	"fmt"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

// needed to setup the holochain environment, not really a test.
func Test(t *testing.T) {
	Register()
}

func TestGob(t *testing.T) {
	g := GobEntry{C: mkTestHeader("myData")}
	v, err := g.Marshal()
	ExpectNoErr(t, err)
	var g2 GobEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t, err)
	sg1 := fmt.Sprintf("%v", g)
	sg2 := fmt.Sprintf("%v", g)
	if sg2 != sg1 {
		t.Error("expected gob match! \n  " + sg2 + " \n  " + sg1)
	}
}

func TestJSONEntry(t *testing.T) {
	/* Not yet implemented or used
	g := JSONEntry{C:Config{Port:8888}}
	v,err := g.Marshal()
	ExpectNoErr(t,err)
	var g2 JSONEntry
	err = g2.Unmarshal(v)
	ExpectNoErr(t,err)
	if g2!=g {t.Error("expected JSON match! "+fmt.Sprintf("%v",g)+" "+fmt.Sprintf("%v",g2))}
	*/
}

func TestJSONSchemaValidator(t *testing.T) {
	d, _ := setupTestService()
	defer cleanupTestDir(d)

	schema := `{
	"title": "Profile Schema",
	"type": "object",
	"properties": {
		"firstName": {
			"type": "string"
		},
		"lastName": {
			"type": "string"
		},
		"age": {
			"description": "Age in years",
			"type": "integer",
			"minimum": 0
		}
	},
	"required": ["firstName", "lastName"]
}`
	ed := EntryDef{Schema: "profile_schema.json"}

	if err := writeFile(d, ed.Schema, []byte(schema)); err != nil {
		panic(err)
	}

	Convey("it should validate JSON entries", t, func() {
		err := ed.BuildJSONSchemaValidator(d)
		So(err, ShouldBeNil)
		So(ed.validator, ShouldNotBeNil)
		profile := `{"firstName":"Eric","lastName":"H-B"}`

		var input interface{}
		if err = json.Unmarshal([]byte(profile), &input); err != nil {
			panic(err)
		}
		err = ed.validator.Validate(input)
		So(err, ShouldBeNil)
		profile = `{"firstName":"Eric"}`
		if err = json.Unmarshal([]byte(profile), &input); err != nil {
			panic(err)
		}

		err = ed.validator.Validate(input)
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldEqual, "validator profile_schema.json failed: object property 'lastName' is required")
	})
}
