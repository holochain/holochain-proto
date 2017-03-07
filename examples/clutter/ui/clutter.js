function send(fn,data,resultFn) {
    $.post(
        "/fn/clutter/"+fn,
        JSON.stringify(data),
        function(response) {
            resultFn(JSON.parse(response));
        }
    ).error(function(response) {
        console.log(response.responseText);
    })
    ;
};

function getProfile() {
    send("get",{what:"nick"},function(data) {
        Clutter.nick = data;
        $("#nick").html(data);
        getPosts(data);
    });
}

function addPost() {
    var now = new(Date);
    var post = {
        message:$('#meow').val(),
        stamp: now.valueOf()
    };
    send("addPost",post,function(data) {
        post.key = data; // save the key of our post to the post
        post.nick = Clutter.nick;
        var id = cachePost(post);
        $("#meows").prepend(makePost(id,post,Clutter.nick));
    });
}

function makePost(id,post) {
    return '<div class="meow" id="'+id+'"><span class="user">'+post.nick+'</span><span class="message">'+post.message+'</span></div>';
}

function getPosts(by) {
    send("get",{what:"posts",whom:by},function(arr) {
        for (var i = 0, len = arr.length; i < len; i++) {
            var post = JSON.parse(arr[i].C);
            post.nick = by;
            var id = cachePost(post);
            displayPosts();
//            $("#meows").prepend(makePost(id,post));
        }
    });
}

function cachePost(p,nick) {
    var id = p.stamp.toString()+nick;
    Clutter.posts[id] = p;
    return id;
}

var Clutter = {posts:{},nick:""};

function displayPosts() {
    var keys = [],
    k, i, len;

    for (k in Clutter.posts) {
        if (Clutter.posts.hasOwnProperty(k)) {
            keys.push(k);
        }
    }

    keys.sort().reverse();

    len = keys.length;

    $("#meows").html("");
    for (i = 0; i < len; i++) {
        k = keys[i];
        var post = Clutter.posts[k];
        $("#meows").append(makePost(k,post));
    }
}
