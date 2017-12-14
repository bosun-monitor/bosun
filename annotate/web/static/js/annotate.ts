/// <reference path="typings/main/ambient/angular/angular.d.ts" />
/// <reference path="typings/main/ambient/jquery/jquery.d.ts" />
/// <reference path="typings/main/definitions/moment/moment.d.ts" />
/// <reference path="typings/main/ambient/underscore/underscore.d.ts" />

declare var moment: any;

var annotateApp = angular.module('annotateApp', [
    'ngRoute',
    'annotateControllers',
    'mgcrea.ngStrap',
]);

var timeFormat = 'YYYY-MM-DDTHH:mm:ssZ';

class Annotation {
    Id: string;
    Message: string;
    StartDate: string; // RFC3999
    EndDate: string; // RFC3999
    CreationUser: string; 
    Url: string;
    Source: string;
    Host: string;
    Owner: string;
    Category: string;
    
    constructor(a?) {
        a = a || {};
        this.Id = a.Id || "";
        this.Message = a.Message || "";
        this.StartDate = a.StartDate || "";
        this.EndDate = a.EndDate || "";
        this.CreationUser = a.CreationUser || getUser() || "";
        this.Url = a.Url || "";
        this.Source = a.Source || "annotate-ui";
        this.Host = a.Host || "";
        this.Owner = a.Owner || getOwner() || "";
        this.Category = a.Category || "";
    }
    setTime() {
        var now = moment().format(timeFormat)
        this.StartDate = now;
        this.EndDate = now;
    }
}



// Reference Struct
// type Annotation struct {
// 	Id           string
// 	Message      string
// 	StartDate    time.Time
// 	EndDate      time.Time
// 	CreationUser string
// 	Url          *url.URL `json:",omitempty"`
// 	Source       string
// 	Host         string
// 	Owner        string
// 	Category     string
// }


annotateApp.config(['$routeProvider', '$locationProvider', '$httpProvider', function($routeProvider, $locationProvider, $httpProvider) {
    $locationProvider.html5Mode(true);
    $routeProvider.
        when('/', {
            title: 'Create',
            templateUrl: 'static/partials/create.html',
            controller: 'CreateCtrl',
        }).
        when('/list', {
            title: 'List',
            templateUrl: 'static/partials/list.html',
            controller: 'ListCtrl',
        }).
        otherwise({
            redirectTo: '/',
        });
}]);

annotateApp.run(['$location', '$rootScope', function($location, $rootScope) {
    // $rootScope.$on('$routeChangeSuccess', function(event, current, previous) {
    // 	$rootScope.title = current.$$route.title;
    // });
}]);

var annotateControllers = angular.module('annotateControllers', [])

annotateControllers.controller('AnnotateCtrl', ['$scope', '$route', '$http', '$rootScope', function($scope, $route, $http, $rootScope) {
    $scope.active = (v) => {
        if (!$route.current) {
            return null;
        }
        if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
            return { active: true };
        }
        return null;
    };
}]);

annotateControllers.controller('CreateCtrl', ['$scope', '$http', '$routeParams', function($scope, $http, $p) {
    if ($p.guid) {
        $http.get('/annotation/' + $p.guid)
            .success((data) => {
                $scope.annotation = new Annotation(data);
            })
            .error((error) => {
                $scope.error = error;
            })
    } else {
        var a = new Annotation();
        a.setTime();
        $scope.annotation = a;
    }
    $http.get('/annotation/values/Owner')
        .success((data) => {
            $scope.owners = data;
        })
    $http.get('/annotation/values/Category')
        .success((data) => {
            $scope.categories = data;
        })
    $http.get('/annotation/values/Host')
        .success((data) => {
            $scope.hosts = data;
        })
    $scope.switch = () => {
        var m = moment.parseZone($scope.annotation.StartDate);
        if (m.zone() == 0) {
            $scope.annotation.StartDate = moment($scope.annotation.StartDate).local().format(timeFormat);
            $scope.annotation.EndDate = moment($scope.annotation.EndDate).local().format(timeFormat);
        } else {
            $scope.annotation.StartDate = moment($scope.annotation.StartDate).utc().format(timeFormat);
            $scope.annotation.EndDate = moment($scope.annotation.EndDate).utc().format(timeFormat);
        }
    }
    $scope.submit = () => {
        var idMissing = $scope.annotation.Id == "";
        if (idMissing && $scope.annotation.CreationUser != "") {
            setUser($scope.annotation.CreationUser);
        }
        if (idMissing && $scope.annotation.Owner != "") {
            setOwner($scope.annotation.Owner);
        }
        $http.post('/annotation', $scope.annotation)
            .success((data) => {
                $scope.annotation = new Annotation(data);
                $scope.error = "";
            })
            .error((error) => {
                $scope.error = error;
            })
    };
}]);

annotateControllers.controller('ListCtrl', ['$scope', '$http', function($scope, $http) {
    $scope.EndDate = moment().format(timeFormat);
    $scope.StartDate = moment().subtract(1, "hours").format(timeFormat);
    $scope.url = (url) => {
        url = url.replace(/.*?:\/\//g, "")
        if (url.length > 20) {
            url = url.substring(0, 20 - 3) + "..."
        }
        return url;
    }
    $scope.active = (a) => {
        var now = moment();
        return moment(a.StartDate).isBefore(now) && moment(a.EndDate).isAfter(now);
    }
    $scope.get = () => {
        var params = "StartDate=" + encodeURIComponent($scope.StartDate) + "&EndDate=" + encodeURIComponent($scope.EndDate);
        $http.get('/annotation/query?' + params)
            .success(function(data) {
                $scope.annotations = data;
            })
            .error(function(error) {
                $scope.status = 'Unable to fetch annotations: ' + error;
            });
    }
    $scope.get();
    $scope.delete = (id) => {
        $http.delete('/annotation/' + id)
            .error((error) => {
                $scope.status = error;
            })
            .success(() => {
                // Remove deleted item from scope model
                $scope.annotations = _.without($scope.annotations, _.findWhere($scope.annotations, { "Id": id }));
            })
    }
    $scope.switch = () => {
        var m = moment.parseZone($scope.StartDate);
        if (m.zone() == 0) {
            $scope.StartDate = moment($scope.StartDate).local().format(timeFormat);
            $scope.EndDate = moment($scope.EndDate).local().format(timeFormat);
        } else {
            $scope.StartDate = moment($scope.StartDate).utc().format(timeFormat);
            $scope.EndDate = moment($scope.EndDate).utc().format(timeFormat);
        }
    }
}]);

// From http://www.quirksmode.org/js/cookies.html
declare function escape(string: string): string;

declare function unescape(string: string): string;

interface Date {
	toGMTString(): string;
}

function createCookie(name, value, days) {
    var expires;
    if (days) {
        var date = new Date();
        date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
        expires = "; expires=" + date.toGMTString();
    }
    else {
        expires = "";
    }
    document.cookie = escape(name) + "=" + escape(value) + expires + "; path=/";
}
function readCookie(name) {
    var nameEQ = escape(name) + "=";
    var ca = document.cookie.split(';');
    for (var i = 0; i < ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) === ' ')
            c = c.substring(1, c.length);
        if (c.indexOf(nameEQ) === 0)
            return unescape(c.substring(nameEQ.length, c.length));
    }
    return null;
}
function eraseCookie(name) {
    createCookie(name, "", -1);
}

function getUser() {
    return readCookie('action-user');
}
function setUser(name) {
    createCookie('action-user', name, 1000);
}

function getOwner() {
    return readCookie('action-owner');
}
function setOwner(name) {
    console.log("set-cookie owner")
    createCookie('action-owner', name, 1000);
}