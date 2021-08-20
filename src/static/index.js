window.addEventListener("load", function(evt) {
    let runBtn = document.getElementById("run")
    let textArea = document.getElementById("code")
    let output = document.getElementById("output")
    let ws
    let running = false

    textArea.onkeydown = function(e){
        if(e.keyCode==9 || e.which==9){
            e.preventDefault();
            var s = this.selectionStart;
            this.value = this.value.substring(0,this.selectionStart) + "\t" + this.value.substring(this.selectionEnd);
            this.selectionEnd = s+1; 
        }
    }

    function clearOutput() {
        output.innerHTML = ""
    }

    if(!ws) {
        var loc = window.location, new_uri
        if (loc.protocol === "https:") {
            new_uri = "wss:"
        } else {
            new_uri = "ws:"
        }
        new_uri += "//" + loc.host
        new_uri += loc.pathname + "ws"
        ws = new WebSocket(new_uri)
        ws.onopen = function(evt) {
            console.log("websocket connection open")
        }
        ws.onclose = function(evt) {
            console.log("websocket connection closed")
            ws = null
        }
        ws.onmessage = function(evt) {
            console.log(evt)
            var msg = JSON.parse(evt.data)
            console.log(msg)
            switch (msg.type) {
                case "started":
                    running = true
                    break;
                case "stopped":
                    running = false
                case "stdout":
                    output.innerHTML += `
                    <span class="stdout">${msg.msg}</span><br>
                    `
                    break;
                case "stderr":
                    output.innerHTML += `
                    <span class="stderr">${msg.msg}</span><br>
                    `
                    break;
                case "error":
                    console.log(msg.msg)
                    running = false
                    break;
                default:
                    console.log("unknown websocket message")
            }
        }
        ws.onerror = function(evt) {
            console.log("error: " + evt.data)
        }
    }

    runBtn.addEventListener("click", function(evt) {
        if(ws && !running) {
            clearOutput()
            ws.send(JSON.stringify({
                type: "run",
                msg: String(textArea.value)
            }))
        } 
    })

    
    window.addEventListener("beforeunload", function(evt) {
        if(ws) {
            ws.send(JSON.stringify({
                type: "close",
                msg: ""
            }))
        }
    });
});