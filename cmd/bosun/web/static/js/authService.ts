/// <reference path="0-bosun.ts" />

class AuthService implements IAuthService {
    private roles: RoleDefs;
    private userPerms: number;
    private username: string;
    private authEnabled: boolean;
    public Init(authEnabled: boolean, username: string, roles: RoleDefs, userPerms: number) {
        this.roles = roles;
        this.username = username;
        this.userPerms = userPerms;
        this.authEnabled = authEnabled;
        this.cleanRoles();
        if (!authEnabled) {
            var cookVal = readCookie("action-user")
            if (cookVal) {
                this.username = cookVal;
            }
        }
    }

    public HasPermission(s: string) {
        for (let p of this.roles.Permissions) {
            if (p.Name == s) {
                return (p.Bits & this.userPerms) != 0
            }
        }
        return true;
    }

    public PermissionsFor(bits: number): string[] {
        if (bits == null) {
            bits = this.userPerms;
        }
        var perms = [];
        for (let p of this.roles.Permissions) {
            if (p.Bits & bits) {
                perms.push(p.Name);
            }
        }
        return perms;
    }

    public RoleFor(bits: number): string {
        if (bits == null) {
            bits = this.userPerms;
        }
        var perms = [];
         for (let r of this.roles.Roles){
              if (r.Bits == bits) {
                 return r.Name;
             }
        }
        return null;
    }

    public GetRoles() {
        return this.roles;
    }
    public Username(u: string) {
        if (!this.authEnabled && angular.isDefined(u)) {
            this.username = u;
            createCookie("action-user", u, 90);
        }
        return this.username
    }
    public GetUsername(): string {
        return this.username
    }
    public Enabled() {
        return this.authEnabled;
    }
    private cleanRoles() {
        //fix admin role that has extra bits corresponding to future permissions.
        //causes bit math to go crazy and overflow. 
        //prevents easily  making tokens that grant unknown future perms too.
        _(this.roles.Roles).each((role) => {
            var mask = 0;
            _(this.roles.Permissions).each((p) => {
                if ((p.Bits & role.Bits) != 0) {
                    mask |= p.Bits
                }
            })
            role.Bits = mask;
        })
    }
}
bosunApp.service("authService", AuthService)

//simple component to show a <username-input> easily
class UsernameInputController {
    static $inject = ['authService'];
    constructor(private auth: IAuthService) {
    }
}
bosunApp.component("usernameInput", {
    controller: UsernameInputController,
    controllerAs: "ct",
    template: '<input type="text"class="form-control"  ng-disabled="ct.auth.Enabled()" ng-model="ct.auth.Username" ng-model-options="{ getterSetter: true }">',
})