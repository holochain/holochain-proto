//expose("addProfile",HC.JSON);
function addProfile(x) {return commit("profile",x);}
expose("addPost",HC.JSON);
function addPost(x) {
    var id = property("_id");
    var nick = property("_agent_name");
    var k = commit("post",x);
    putmeta(id,k,"_post_by_"+nick);
    return k;
}
expose("users",HC.JSON)
function users() {
    debug("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
    return getmeta(property("_id"),"users")
}

expose("get",HC.JSON)
function get(i) {
    debug("get")
    if (i.what == "nick") {
        return property("_agent_name");
    }
    return {}
}

function genesis() {
    var id = property("_id");
    debug("id is "+id)
    var k = addProfile({nick:property("_agent_name")});
    var err = putmeta(id,k,"users");
    if (!err) {
        return true;
    }
    return true;
}

function validate(entry_type,entry,meta) {
    if (entry_type=="post") {
        var l = entry.message;
        if (l>0 && l<256) {return true;}
        return false;
    }
    if (entry_type=="profile") {
        return true;
    }
    return false;
}
