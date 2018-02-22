function Library() {
}

Library.prototype.function_heading = function (name, parms) {
    return `
        <h2 class="function-heading">` + name + `</h2>
    `
}

Library.prototype.code_example_heading = function (name) {
}

Library.prototype.code_example_block_header = function (name, exampleName) {
    return `
    <pre><code class="code-example" id="code-example-` + name + `-` + exampleName + `">
    `
}

Library.prototype.code_example_block_footer = function (name, exampleName) {
    return `
        </code></pre>
    `
}

Library.prototype.function_link = function (name, exampleName) {
    return `
        <a href="#">` + name + `</a>
    `
}

module.exports = new Library();

