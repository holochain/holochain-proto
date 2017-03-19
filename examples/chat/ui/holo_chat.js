
 function getMyProfile() {
   $.get("/fn/profiles/myProfile", "", function(profile){
     $("#title-username").text(JSON.parse(profile).firstName)
   });
 }

 function getRooms() {
   $.get("/fn/rooms/listRooms", "", function(rooms){
     rooms = JSON.parse(rooms)
     $("#rooms").empty()
     for(i=0;i<rooms.length;i++){
       $("#rooms").append("<li>"+rooms[i].name+"</li>")
     }
   });
 };

 function addRoom() {
   var room = {
     name: $("#room-name-input").val(),
     purpose: "..."
   }
   $("#room-name-input").val('')
   $.post("/fn/rooms/newRoom", JSON.stringify(room), getRooms)
 }

 function selectRoom(event) {
   $("#rooms li").removeClass("selected-room")
   $(this).addClass("selected-room")
 }



 function doRegister(){
   var arg = {
     username: $("#signupUsername").val(),
     firstName: $("#signupFirstname").val(),
     lastName: $("#signupLastname").val(),
     email: $("#signupEmail").val()
   };
   console.log('signup clicked');
   $.post("/fn/profiles/register", JSON.stringify(arg),
     function(hash) {
       console.log('register: '+hash)
       $.post("/fn/profiles/isRegistered", "",
           function(registered) {
               console.log('registered: '+registered)
               if(JSON.parse(registered)) {
                 getMyProfile()
                 $('#registerDialog').modal('hide');
               } else {
                 $('#registerDialog').modal('show');
               }
           });
     },
     "json"
   );
 }


 $(window).ready(function() {
    $.post("/fn/profiles/isRegistered", "",
        function(registered) {

            if(!JSON.parse(registered)){
                $('#registerDialog').modal('show');
            } else {
                getMyProfile()
                getRooms()
            }

        }
    ).error(function(response) {
        $("#messages").html(response.responseText)
    });

    $("#signupButton").click(doRegister);
    $("#room-name-button").click(addRoom);
    $("#rooms").on("click", "li", selectRoom);
 });
