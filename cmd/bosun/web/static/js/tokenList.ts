/// <reference path="0-bosun.ts" />

class TokenListController {
    tokens: Array<Token>;
    status: string ;

    delete = (hash: string) => {
        this.status = "Deleting..."
        this.$http.delete("/api/tokens?hash=" + encodeURIComponent(hash))
            .then(() => {
                this.status = "";
                this.load();
            }, (err) => {
                this.status = 'Unable to delete token: ' + err;
            })
    }

    load = () =>{
        this.status = "Loading..."
        this.$http.get("/api/tokens").then(
            (resp: ng.IHttpPromiseCallbackArg<Token[]>) => {
                _(resp.data).forEach((tok) => {
                    tok.LastUsed = moment.utc(tok.LastUsed);
                    tok.Permissions = this.auth.PermissionsFor(tok.Role);
                    tok.RoleName = this.auth.RoleFor(tok.Role) || ("" + tok.Permissions.length+" Permissions");
                })
                this.tokens = resp.data;
                this.status = "";
            }, (err) => {
                this.status = 'Unable to fetch tokens: ' + err;
            }
        )
    }

    permList = (tok: Token): string =>{
        //HACK: return html string for popover. angular-strap has bad api for this
        var h = `<div class="popover" tabindex="-1">
        <div class="arrow"></div>
        <div class="popover-content"><ul>`
        var perms = this.auth.PermissionsFor(tok.Role);
        for (var i = 0; i< perms.length; i++){
            var p = perms[i];
            var open = "<strong>";
            var close = "</strong>";
            h += "<li>"+open+p+close+"</li>";
        }
        h += `</ul></div></div>`
        return h;
    }

    static $inject = ['$http', "authService"];
    constructor(private $http: ng.IHttpService, private auth: IAuthService) {
        this.load();
    }
}

bosunApp.component('tokenList', {
    controller: TokenListController,
    controllerAs: "ct",
    template: `
<div class="alert alert-danger" ng-show="ct.status">{{ct.status}}</div>
<h2>Access Tokens</h2>
    <table class="table table-striped">
        <thead>
        <tr>
            <th>ID</th>
            <th>User</th>
            <th>Description</th>
            <th>Permissions</th>
            <th>Last Used</th>
            <th></th>
        </tr>
        </thead>
        <tbody>
        <tr  ng-repeat="tok in ct.tokens | orderBy:'-LastUsed'">
            <td>{{tok.Hash | limitTo: 6}}</td>
            <td>{{tok.User}}</td>
            <td>{{tok.Description}}</td>
            <td>
                <a data-template="{{ct.permList(tok)}}" 
                data-animation="am-flip-x" 
                data-trigger="hover"
                data-auto-close="1" bs-popover>{{tok.RoleName}}</a>
 
            </td>
            <td><span ng-if="tok.LastUsed.year() > 2000" ts-since="tok.LastUsed"></span> <span ng-if="tok.LastUsed.year() <= 2000">Never</span></td>
            <td><a class='btn btn-danger fa fa-trash' ng-click='ct.delete(tok.Hash)'></a></td>
        </tr>
        </tbody>
    </table>
    <a class='btn btn-primary' href='/tokens/new'><span class='fa fa-plus'/> Create new token</a>
`});
