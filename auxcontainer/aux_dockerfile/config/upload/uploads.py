from flask import Flask, request
from brain.binary import put_buffer

app = Flask(__name__)


# curl --form "file=@localfile" http://localhost:10090/newname
@app.route("/<name>", methods=['POST'])
def adding(name):
    if request.method == 'POST':
        file = request.files['file']
        put_buffer(name, file.read())
    return ""

if __name__ == "__main__":
    app.run(host='0.0.0.0', port=10090, debug=True)
