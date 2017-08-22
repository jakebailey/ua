// This file is automatically generated by qtc from "spec.qtpl".
// See https://github.com/valyala/quicktemplate for details.

//line spec.qtpl:1
package templates

//line spec.qtpl:1
import (
	qtio422016 "io"

	qt422016 "github.com/valyala/quicktemplate"
)

//line spec.qtpl:1
var (
	_ = qtio422016.Copy
	_ = qt422016.AcquireByteBuffer
)

//line spec.qtpl:1
func StreamSpec(qw422016 *qt422016.Writer, url string) {
	//line spec.qtpl:1
	qw422016.N().S(`
<!doctype html>
<html>

<head>
    
</head>

<body>
    <form action="" method="post" id="form">
        <div>
            <label for="name">Assignment name:</label>
            <input type="text" id="name" name="assignment_name" value="test"/>
        </div>
        <div>
            <label for="seed">Seed:</label>
            <input type="text" id="seed" name="seed" value="1234"/>
        </div>
        <div>
            <label for="data">Message:</label>
            <textarea id="data" name="data">{"NetID": "jbbaile2"}</textarea>
        </div>
        <button type="submit">Submit</button>
    </form>

    <div id="result"></div>

    <script src="https://ajax.googleapis.com/ajax/libs/jquery/3.2.1/jquery.min.js"></script>

    <script>
        $("#form").submit(function(event) {
            event.preventDefault();

            var postData = {
                assignmentName: $("#name").val(),
                seed: $("#seed").val(),
                data: JSON.parse($("#data").val())
            };

            $.ajax({
                url: '`)
	//line spec.qtpl:41
	qw422016.E().S(url)
	//line spec.qtpl:41
	qw422016.N().S(`',
                type: 'POST',
                contentType: 'application/json',
                data: JSON.stringify(postData),
                dataType: 'json',
                success: function(data) {
                    $("#result").append('<a href="/term/' + data.id + '" target="_blank">' + data.id + '</a>');
                }
            });
        });
    </script>
</body>

</html>
`)
//line spec.qtpl:55
}

//line spec.qtpl:55
func WriteSpec(qq422016 qtio422016.Writer, url string) {
	//line spec.qtpl:55
	qw422016 := qt422016.AcquireWriter(qq422016)
	//line spec.qtpl:55
	StreamSpec(qw422016, url)
	//line spec.qtpl:55
	qt422016.ReleaseWriter(qw422016)
//line spec.qtpl:55
}

//line spec.qtpl:55
func Spec(url string) string {
	//line spec.qtpl:55
	qb422016 := qt422016.AcquireByteBuffer()
	//line spec.qtpl:55
	WriteSpec(qb422016, url)
	//line spec.qtpl:55
	qs422016 := string(qb422016.B)
	//line spec.qtpl:55
	qt422016.ReleaseByteBuffer(qb422016)
	//line spec.qtpl:55
	return qs422016
//line spec.qtpl:55
}
