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
        $("#nick").html(data);
    });
}

function addPost() {
    var post = $('#meow').val();
    send("addPost",post,function(data) {
        $("#meows").prepend('<div class="meow" key="'+data+'">'+post+'</div>');
    });
}
