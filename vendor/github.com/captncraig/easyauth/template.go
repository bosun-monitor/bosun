package easyauth

const loginTemplate = `
<html>
<head>
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">
<style>
.tab-content {
    border-left: 1px solid #ddd;
    border-right: 1px solid #ddd;
}
.nav-tabs {
    margin-bottom: 0;
}
</style>
<script src="http://code.jquery.com/jquery-3.1.1.min.js" integrity="sha256-hVVnYaiADRTO2PzUGmuLJr8BLUSjGIZsDYGmIJLv2b8=" crossorigin="anonymous"></script>
<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/js/bootstrap.min.js" integrity="sha384-Tc5IQib027qvyjSMfHjOMaLkfuWVxZxUPnCJA7l2mCWNIpG9mGCD8wGNIcPD7Txa" crossorigin="anonymous"></script>
</head>
<body>
<div class="container">
    {{if .Message}}<div class="alert alert-danger" role="alert">{{.Message}}</div>{{end}}
    <div class='well' style='width:500px; margin:auto;margin-top:45px;'>
        <h2>Login</h2>
        {{if gt (len .Auth.FormProviders) 1}}
        <ul class='nav nav-tabs nav-justified'>
            {{range $index, $p := .Auth.FormProviders}}
            <li role="presentation" {{if eq $index 0}}class='active'{{end}}>
                <a href="#{{$p.Name}}" role="tab" data-toggle="tab">{{$p.Name}}</a>
            </li>
            {{end}}
        </ul>
        {{end}}
        <div class="tab-content">
            {{range $index, $p := .Auth.FormProviders}}
                <div role="tabpanel" class="tab-pane{{if eq $index 0}} active{{end}}" id="{{$p.Name}}" style='background-color: white'>
                    <form style='padding:10px;' action="./{{$p.Name}}" method="post">
                        {{range $p.Provider.GetRequiredFields}}
                            <label for="{{.}}">{{.}}</label>
							{{if eq . "Password"}}
							<input type="password" id="{{.}}" name="{{.}}" class="form-control" placeholder="{{.}}" required>
							{{else}}
                            <input type="text" id="{{.}}" name="{{.}}" class="form-control" placeholder="{{.}}" required>
							{{end}}
                        {{end}}
                        <button class="btn btn-primary" type="submit" style="margin-top: 15px">Sign in</button>
                    </form>
                </div>
            {{end}}
        </div>
    </div>
</div> 
</body>
</html>`
