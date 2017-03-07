expose("addPost",HC.JSON);
function addPost(x) {
    var id = property("_id");
    var nick = property("_agent_name");
    var k = commit("post",x);
    put(k);
    putmeta(id,k,"_post_by_"+nick);
    return k;
}

expose("get",HC.JSON);
function get(i) {
    debug("get");
    var id = property("_id");
    if (i.what == "nick") {
        return property("_agent_name");
    }
    else if (i.what == "posts") {
        return getmeta(id,"_post_by_"+i.whom);
    }
    else if (i.what == "follows") {
        return getmeta(id,"_follows_by_"+i.whom);
    }
    else if (i.what == "users") {
        return getmeta(id,"users");
    }
    return {};
}

expose("follow",HC.JSON);
function follow(x) {
    var id = property("_id");
    var nick = property("_agent_name");
    var k = commit("follow",x);
    put(k);
    putmeta(id,k,"_follows_by_"+nick);
    return k;
}

// callbacks -----------------------------------------------------
function genesis() {
    var id = property("_id");
    debug("id is "+id);
    var k = commit("profile",{nick:property("_agent_name")});
    put(k);
    var err = putmeta(id,k,"users");
    if (!err) {
        return true;
    }
    return true;
}

function validate(entry_type,entry,meta) {
    debug("validate: "+entry_type);
    if (entry_type=="post") {
        var l = entry.message.length;
        if (l>0 && l<256) {return true;}
        return false;
    }
    if (entry_type=="profile") {
        return true;
    }
    if (entry_type=="follow") {
        return true;
    }
    return false;
}
