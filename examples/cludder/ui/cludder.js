var Cludder = {posts:{},users:{},handles:{},follows:{},handle:"",me:""};

function getHandle(who,fn) {
    send("getHandle",who,function(data) {
        cacheUser({handle:data,hash:who});
        if (fn!=undefined) {
            fn(data);
        }
    });
}

function getMyHandle(fn) {
    getHandle(Cludder.me,function(data){
        Cludder.handle = data;
        $("#handle").html(data);
        if (fn!=undefined) {
            fn();
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
        Cludder.me = me;
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
        post.handle = Cludder.handle;
        var id = cachePost(post);
        $("#meows").prepend(makePostHTML(id,post,Cludder.handle));
    });
}

function follow(w) {
    send("follow",w,function(data) {
        cacheFollow(w);
    });
}

function makePostHTML(id,post) {
    var d = Date(post.stamp);
    return '<div class="meow" id="'+id+'"><div class="stamp">'+d+'</div><div class="user">'+post.handle+'</div><div class="message">'+post.message+'</div></div>';
}

function makeUserHTML(user) {
    return '<div class="user">'+user.handle+'</div>';
}

function makeResultHTML(result) {
    var id;
    return '<div class="search-result" id="'+id+'"><div class="user">'+result.handle+'</div></div>';
}

function getMyPosts() {
    getPosts(Cludder.me);
}

function getPosts(by) {
    send("getPostsBy",by,function(arr) {

        arr = JSON.parse(arr);
        console.log("arr: " + JSON.stringify(arr));

        // if we actually get something, then get the handle and
        // add it to the posts objects before caching.
        var len = len = arr.length;
        if (len > 0) {
            var postsFn = function(author_handle) {
                for (var i = 0; i < len; i++) {
                    console.log("arr[i]: " + JSON.stringify(arr[i]));
                    var post = JSON.parse(arr[i].post);
                    post.handle = author_handle;
                    var id = cachePost(post);
                    displayPosts();
                    //            $("#meows").prepend(makePost(id,post));
                }
            };
            var user = Cludder.handles[by];
            if (user == undefined) {
                send("getHandle", by, postsFn);
            }
            else {
                postsFn(user.handle);
            }
        }
    });
}

function cachePost(p) {
    //console.log("Caching:"+JSON.stringify(p));
    var id = p.stamp.toString()+p.handle;
    Cludder.posts[id] = p;
    return id;
}

function cacheUser(u) {
    Cludder.users[u.handle] = u;
    Cludder.handles[u.hash] = u;
}

function cacheFollow(f) {
    console.log("caching: "+f);
    Cludder.follows[f] = true;
}

function uncacheFollow(f) {
    console.log("uncaching: "+f);
    delete Cludder.follows[f];
}

function makeFollowingHTML(handle) {
    return "<div class='following-handle'><span class='handle'>"+handle+'</span><button type="button" class="close" aria-label="Close" onclick="unfollow(this);"><span aria-hidden="true">&times;</span></button></div>';
}

function displayFollowing() {
    var handles = [];
    var following = Object.keys(Cludder.follows);
    var len = following.length;
    for (var i = 0; i < len; i++) {
        var user = Cludder.handles[following[i]];
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

function displayPosts() {
    var keys = [],
    k, i, len;

    for (k in Cludder.posts) {
        if (Cludder.posts.hasOwnProperty(k)) {
            keys.push(k);
        }
    }

    keys.sort().reverse();

    len = keys.length;

    $("#meows").html("");
    for (i = 0; i < len; i++) {
        k = keys[i];
        var post = Cludder.posts[k];
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
        if (tab == cur) {w
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
    var user = Cludder.users[handle].hash;
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

    getProfile();
});
