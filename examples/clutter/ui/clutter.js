var App = {posts:{},users:{},handles:{},follows:{},handle:"",me:""};

function getHandle(who,callbackFn) {
    send("getHandle",who,function(handle) {
        cacheUser({handle:handle,hash:who});
        if (callbackFn!=undefined) {
            callbackFn(who,handle);
        }
    });
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

function getFollow(who,type) {
    send("getFollow",JSON.stringify({from:who,type:type}),function(data) {
        var j =  JSON.parse(data);
        var following = j.result;
        if (following != undefined) {

            var len = following.length;
            for (var i = 0; i < len; i++) {
                cacheFollow(following[i]);
            }
        }
    });
}

function getProfile() {
    send("appProperty","App_Key_Hash", function(me) {
        App.me = me;
        getMyHandle(getMyPosts);
        getFollow(me,"following");
    });
}

function addPost() {
    var now = new(Date);
    var post = {
        message:$('#meow').val(),
        stamp: now.valueOf()
    };
    send("post",JSON.stringify(post),function(data) {
        post.key = data; // save the key of our post to the post
        post.author = App.me;
        var id = cachePost(post);
        $("#meows").prepend(makePostHTML(id,post,App.handle));
    });
}

function follow(w) {
    send("follow",w,function(data) {
        cacheFollow(w);
    });
}

function makePostHTML(id,post) {
    var d = Date(post.stamp);
    var author = App.handles[post.author];
    var handle;
    if (author == undefined) {
        handle = post.author;
    } else {
        handle = author.handle;
    }
    return '<div class="meow" id="'+id+'"><div class="stamp">'+d+'</div><div class="user">'+handle+'</div><div class="message">'+post.message+'</div></div>';
}

function makeUserHTML(user) {
    return '<div class="user">'+user.handle+'</div>';
}

function makeResultHTML(result) {
    var id;
    return '<div class="search-result" id="'+id+'"><div class="user">'+result.handle+'</div></div>';
}

function getMyPosts() {
    getPosts([App.me]);
}

function getPosts(by) {

    // check to see if we have the user's handles
    for (var i=0;i<by.length;i++) {
        var author = by[i];
        var handle = App.handles[author];
        if (handle == undefined) {
            send("getHandle", author);
        }
    }
    send("getPostsBy",JSON.stringify(by),function(arr) {
        arr = JSON.parse(arr);
        console.log("posts: " + JSON.stringify(arr));

        // if we actually get something, then get the handle and
        // add it to the posts objects before caching.
        var len = len = arr.length;
        if (len > 0) {
            for (var i = 0; i < len; i++) {
                console.log("arr[i]: " + JSON.stringify(arr[i]));
                var post = JSON.parse(arr[i].post);
                post.author = arr[i].author;
                var id = cachePost(post);
                //            $("#meows").prepend(makePost(id,post));
            }
        }
        displayPosts();
    });
}

function cachePost(p) {
    //console.log("Caching:"+JSON.stringify(p));
    var id = p.stamp;
    App.posts[id] = p;
    return id;
}

function cacheUser(u) {
    App.users[u.handle] = u;
    App.handles[u.hash] = u;
}

function cacheFollow(f) {
    console.log("caching: "+f);
    App.follows[f] = true;
}

function uncacheFollow(f) {
    console.log("uncaching: "+f);
    delete App.follows[f];
}

function makeFollowingHTML(handle) {
    return "<div class='following-handle'><span class='handle'>"+handle+'</span><button type="button" class="close" aria-label="Close" onclick="unfollow(this);"><span aria-hidden="true">&times;</span></button></div>';
}

function displayFollowing() {
    var handles = [];
    var following = Object.keys(App.follows);
    var len = following.length;
    for (var i = 0; i < len; i++) {
        var user = App.handles[following[i]];
        if (user != undefined) {
            handles.push(user.handle);
        }
    }
    handles.sort();
    $("#following").html("");
    len = handles.length;
    for (i = 0; i < len; i++) {
        $("#following").append(makeFollowingHTML(handles[i]));
    }
}

function displayPosts(filter) {
    var keys = [],
    k, i, len;

    for (k in App.posts) {
        if (filter != undefined) {
            if (filter.includes(App.posts[k].handle)) {
                keys.push(k);
            }
        } else {
            keys.push(k);
        }
    }

    keys.sort().reverse();

    len = keys.length;

    $("#meows").html("");
    for (i = 0; i < len; i++) {
        k = keys[i];
        var post = App.posts[k];
        $("#meows").append(makePostHTML(k,post));
    }
}

function doFollow() {
    var handle = $("#followHandle").val();

    send("getAgent",handle,function(data) {
        if (data != "") {
            follow(data);
        }
        else {
            alert(handle+" not found");
        }
        $('#followDialog').modal('hide');
    });
}

function doSearch() {
    $('#search-results').fadeIn();
    $("#people-results").html("");
    $("#people-results").append(makeResultHTML({handle:"Bob Smith!"}));
}

function hideSearchResults() {
    $('#search-results').fadeOut();
}

function searchTab(tab) {
    var tabs = $('.search-results-data');
    var len = tabs.length;
    for (i = 0; i < len; i++) {
        var t= tabs[i];
        var tj = $(t);
        var cur = t.id.split("-")[0];
        var tabj = $("#"+cur+"-tab");
        if (tab == cur) {
            tj.slideToggle();
            tabj.addClass('active-tab');
        }
        else {
            tj.slideToggle();
            tabj.removeClass('active-tab');
        }
    }
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

function openFollow() {
    $("#followHandle").val("");
    displayFollowing();
    $('#followDialog').modal('show');
}

function openSetHandle() {
    $('#setHandleDialog').modal('show');
}

function unfollow(button) {
    // pull the handle out from the HTML
    var handle = $(button).parent().find('.handle')[0].innerHTML;
    var user = App.users[handle].hash;
    send("unfollow",user,function(data) {
        uncacheFollow(user);
        $('#followDialog').modal('hide');
    });
}

$(window).ready(function() {
    $("#submitFollow").click(doFollow);
    $('#followButton').click(openFollow);
    $("#handle").on("click", "", openSetHandle);
    $('#setHandleButton').click(doSetHandle);
    $('#search-results.closer').click(hideSearchResults);
    getProfile();
});
