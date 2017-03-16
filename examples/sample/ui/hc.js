
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
