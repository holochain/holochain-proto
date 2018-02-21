const ejs = require('ejs');
const library = require("./library.js")

ejs.renderFile("ejs/function-commit.ejs", { library: library }, { debug: false }, function (err, html) {
    if (err) throw err;

    var fs = require('fs');
    fs.writeFile("html/function-commit.html", html, function(err) {
        if(err) {
            return console.log(err);
        }
    
        console.log("html/function-commit.html");
    }); });
