var App = {users:{},handles:{},handle:"",me:""};

function getHandle(who,callbackFn) {
    send("getHandle",who,function(handle) {
        cacheUser(handle,who);
        if (callbackFn!=undefined) {
            callbackFn(who,handle);
        }
    });
}


function getHandles(callbackFn) {
    send("getHandles","{}",function(handles) {
        handles = JSON.parse(handles);
        for (var i=0;i <handles.length;i++) {
            var handle = handles[i].handle;
            var agent = handles[i].agent;
            cacheUser(handle,agent);
        }
        updatePlayers();
        if (callbackFn!=undefined) {
            callbackFn(handles);
        }
    });
}

function makePlayerHTML(user) {
    return "<li data-id=\""+user.hash+"\""+
        "data-name=\""+user.handle+"\">"+
        user.handle+
        "</li>";
}

function updatePlayers() {
    $("#players").empty();
    for (var handle in App.users) {
        if (App.users.hasOwnProperty(handle)) {
            $("#players").append(makePlayerHTML(App.users[handle]));
        }
    }
    if (App.activePlayer) {
        setActivePlayer();
    }
}

function cacheUser(handle,agent) {
    var u = {handle:handle,hash:agent};
    App.users[handle] = u;
    App.handles[agent] = u;
}

function getMyHandle(callbackFn) {
    getHandle(App.me,function(hash,handle){
        App.handle = handle;
        $("#handle").html(handle);
        if (callbackFn!=undefined) {
            callbackFn();
        }
    });
}

function getProfile() {
    send("appProperty","App_Key_Hash", function(me) {
        App.me = me;
        getMyHandle();
    });
}

function getUserHandle(user) {
    var author = App.handles[user];
    var handle;
    if (author == undefined) {
        handle = user;
    } else {
        handle = author.handle;
    }
    return handle;
}

function doSetHandle() {
    var handle = $("#myHandle").val();

    send("newHandle",handle,function(data) {
        if (data != "") {
            getMyHandle();
        }
        $('#setHandleDialog').modal('hide');
    });
}

function openSetHandle() {
    $('#setHandleDialog').modal('show');
}

function selectPlayer(event) {
    $("#players li").removeClass("selected-player");
    App.activePlayer = $(this).data('id');
    setActivePlayer();
}

function setActivePlayer() {
    var elem = $("#players li[data-id="+App.activePlayer+"]");
    $(elem).addClass("selected-player");
    $("#tosses-header").text("Tosses with "+$(elem).data("name"));
    //getTosses()
}

function confirmToss(toss) {
    // TODO add toss caching
    send("confirmToss",toss,function(result) {
        alert(result);
    });
}

function requestToss() {
    if (!App.activePlayer) {
        alert("pick a player first!");
    }
    else {
        send("requestToss",JSON.stringify({"agent":App.activePlayer}),function(result) {
            result = JSON.parse(result);
            confirmToss(result.toss);
        });
    }
}

$(window).ready(function() {
    $("#handle").on("click", "", openSetHandle);
    $('#setHandleButton').click(doSetHandle);
    $("#players").on("click", "li", selectPlayer);
    $("#req-toss-button").click(requestToss);
    getProfile();
    setInterval(getHandles, 2000);
});
