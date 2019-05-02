/// <reference path="0-bosun.ts" />

class TokenListController {
    tokens: Array<Token>;
    status: string ;
    deleteTarget: string;

    delete = () => {
        this.status = "Deleting..."
        this.$http.delete("/api/tokens?hash=" + encodeURIComponent(this.deleteTarget))
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
    templateUrl : '/static/partials/tokenList.html'});
