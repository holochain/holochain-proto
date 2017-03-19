// Copyright (C) 2013-2017, The MetaCurrency Project (Eric Harris-Braun, Arthur Brock, et. al.)
// Use of this source code is governed by GPLv3 found in the LICENSE file
//----------------------------------------------------------------------------------------

// constant assests

package holochain

const (
	SampleHTML = `
<html>
  <head>
    <title>Test</title>
    <script type="text/javascript" src="http://code.jquery.com/jquery-latest.js"></script>
    <script type="text/javascript" src="/hc.js">
    </script>
  </head>
  <body>
    <select id="zome" name="zome">
      <option value="zySampleZome">zySampleZome</option>
      <option value="jsSampleZome">jsSampleZome</option>
    </select>
    <select id="fn" name="fn">
      <option value="addEven">addEven</option>
      <option value="getProperty">getProperty</option>
      <option value="addPrime">addPrime</option>
    </select>
    <input id="data" name="data">
    <button onclick="send();">Send</button>
    send an even number and get back a hash, send and odd end get a error

    <div id="result"></div>
    <div id="err"></div>
  </body>
</html>`
	SampleJS = `
     function send() {
         $.post(
             "/fn/"+$('select[name=zome]').val()+"/"+$('select[name=fn]').val(),
             $('#data').val(),
             function(data) {
                 $("#result").html("result:"+data)
                 $("#err").html("")
             }
         ).error(function(response) {
             $("#err").html(response.responseText)
             $("#result").html("")
         })
         ;
     };
`
)

var SampleUI = map[string]string{
	"index.html": SampleHTML,
	"hc.js":      SampleJS,
}
