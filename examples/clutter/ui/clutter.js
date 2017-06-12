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

function getFollow(who,type,callbackFn) {
    send("getFollow",JSON.stringify({from:who,type:type}),function(data) {
        var j =  JSON.parse(data);
        var following = j.result;
        if (following != undefined) {
            var len = following.length;
            for (var i = 0; i < len; i++) {
                cacheFollow(following[i]);
            }
            if (callbackFn!=undefined) {
                callbackFn();
            }
        }
    });
}

function getProfile() {
    send("appProperty","App_Key_Hash", function(me) {
        App.me = me;
        getMyHandle();
        getFollow(me,"following",getMyFeed);
    });
}

function addPost() {
    var now = new(Date);
    var post = {
        message:$('#meow').val(),
        stamp: now.valueOf()
    };
    send("post",JSON.stringify(post),function(data) {
        post.key = JSON.parse(data); // save the key of our post to the post
        post.author = App.me;
        var id = cachePost(post);
        $("#meows").prepend(makePostHTML(id,post,App.handle));
    });
}

function doEditPost() {
    var now = new(Date);
    var id = $('#postID').val();
    var post = {
        message:$('#editedMessage').val(),
        stamp: now.valueOf()
    };
    $('#editPostDialog').modal('hide');
    send("postMod",JSON.stringify({hash:App.posts[id].key,post:post}),function(data) {
        post.key = JSON.parse(data); // save the key of our post to the post
        post.author = App.me;
        $("#"+id).remove();
        id = cachePost(post);
        $("#meows").prepend(makePostHTML(id,post,App.handle));
    });
}

function follow(w) {
    send("follow",w,function(data) {
        cacheFollow(w);
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

function makePostHTML(id,post) {
    var d = new Date(post.stamp);
    var handle = getUserHandle(post.author);
    return '<div class="meow" id="'+id+'"><a class="meow-edit" href="#" onclick="openEditPost('+id+')">edit</a><div class="stamp">'+d+'</div><a href="#" class="user" onclick="showUser(\''+post.author+'\');">'+handle+'</a><div class="message">'+post.message+'</div></div>';
}

function makeUserHTML(user) {
    return '<div class="user">'+user.handle+'</div>';
}

function makeResultHTML(result) {
    var id;
    return '<div class="search-result" id="'+id+'"><div class="user">'+result.handle+'</div></div>';
}

function getUserPosts(user) {
    getPosts([user]);
}

function getMyFeed() {
    var users = Object.keys(App.follows);
    if (!users.includes(App.me)) {
        users.push(App.me);
    }
    getPosts(users);
}

function getPosts(by) {

    // check to see if we have the author's handles
    for (var i=0;i<by.length;i++) {
        var author = by[i];
        var handle = App.handles[author];
        if (handle == undefined) {
            getHandle(author);
        }
    }
    send("getPostsBy",JSON.stringify(by),function(arr) {
        arr = JSON.parse(arr);
        console.log("posts: " + JSON.stringify(arr));

        var len = len = arr.length;
        if (len > 0) {
            for (var i = 0; i < len; i++) {
                var post = JSON.parse(arr[i].post);
                post.author = arr[i].author;
                var id = cachePost(post);
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

function openEditPost(id) {
    $("#editedMessage").val(App.posts[id].message);
    $('#postID').val(id);
    $('#editPostDialog').modal('show');
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

function showUser(user) {
    $('#meow-form').fadeOut();
    $('#user-header').html(getUserHandle(user));
    $('#user-header').fadeIn();
    App.posts={};
    getPosts([user]);
}

function showFeed() {
    $('#meow-form').fadeIn();
    $('#user-header').fadeOut();
    App.posts={};
    getMyFeed();
}

$(window).ready(function() {
    $("#submitFollow").click(doFollow);
    $('#followButton').click(openFollow);
    $("#handle").on("click", "", openSetHandle);
    $('#setHandleButton').click(doSetHandle);
    $('#search-results.closer').click(hideSearchResults);
    $('#user-header').click(showFeed);
    $('#editPostButton').click(doEditPost);

    getProfile();
});
