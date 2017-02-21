/// <reference path="moment.d.ts" />
/// <reference path="moment-duration-format.d.ts" />

//Represents an auth token
class Token {
    public Hash: string;
    public Description: string = "";
    public Role: number = 0;
    public User: string = "";
    public LastUsed: Moment;

    public Permissions: string[];
    public RoleName: string;
}

//metadata about a single role or permission
class BitMeta {
    public Bits: number;
    public Name: string;
    public Desc: string;
    public Active: boolean;
}

//all roles/permissions for bosun
class RoleDefs {
    public Permissions: Array<BitMeta>;
    public Roles: Array<BitMeta>;
}