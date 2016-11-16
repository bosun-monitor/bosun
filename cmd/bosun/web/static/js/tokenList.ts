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
            (resp: ng.IHttpPromiseCallbackArg<Array<Token>>) => {
                _(resp.data).forEach((tok) => {
                    tok.LastUsed = moment.utc(tok.LastUsed)
                })
                this.tokens = resp.data;
                this.status = "";
            }, (err) => {
                this.status = 'Unable to fetch tokens: ' + err;
            }
        )
    }

    static $inject = ['$http'];
    constructor(private $http: ng.IHttpService) {
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
            <th>Last Used</th>
        </tr>
        </thead>
        <tbody>
        <tr  ng-repeat="tok in ct.tokens">
            <td>{{tok.Hash | limitTo: 6}}</td>
            <td>{{tok.User}}</td>
            <td>{{tok.Description}}</td>
            <td><span ng-show="tok.LastUsed.year() > 2000" ts-since="tok.LastUsed"></span> <span ng-show="tok.LastUsed.year() <= 2000">Never</span></td>
            <td><a class='btn btn-danger glyphicon glyphicon-trash' ng-click='ct.delete(tok.Hash)'></a></td>
        </tr>
        </tbody>
    </table>
    <a class='btn btn-primary' href='/tokens/new'><span class='glyphicon glyphicon-plus'/> Create new token</a>
`});
