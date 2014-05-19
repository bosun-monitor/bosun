/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="rickshaw.d.ts" />
/// <reference path="d3.d.ts" />

var tsafApp = angular.module('tsafApp', [
	'ngRoute',
	'tsafControllers',
	'mgcrea.ngStrap',
	'ngSanitize',
]);

tsafApp.config(['$routeProvider', '$locationProvider', function($routeProvider: ng.route.IRouteProvider, $locationProvider: ng.ILocationProvider) {
	$locationProvider.html5Mode(true);
	$routeProvider.
		when('/', {
			title: 'Dashboard',
			templateUrl: 'partials/dashboard.html',
			controller: 'DashboardCtrl',
		}).
		when('/items', {
			title: 'Items',
			templateUrl: 'partials/items.html',
			controller: 'ItemsCtrl',
		}).
		when('/expr', {
			title: 'Expression',
			templateUrl: 'partials/expr.html',
			controller: 'ExprCtrl',
		}).
		when('/egraph', {
			title: 'Expression Graph',
			templateUrl: 'partials/egraph.html',
			controller: 'EGraphCtrl',
		}).
		when('/graph', {
			title: 'Graph',
			templateUrl: 'partials/graph.html',
			controller: 'GraphCtrl',
		}).
		when('/host', {
			title: 'Host View',
			templateUrl: 'partials/host.html',
			controller: 'HostCtrl',
			reloadOnSearch: false,
		}).
		when('/rule', {
			title: 'Rule',
			templateUrl: 'partials/rule.html',
			controller: 'RuleCtrl',
		}).
		when('/silence', {
			title: 'Silence',
			templateUrl: 'partials/silence.html',
			controller: 'SilenceCtrl',
		}).
		when('/test_template', {
			title: 'Test Template',
			templateUrl: 'partials/test_template.html',
			controller: 'TestTemplateCtrl',
		}).
		when('/config', {
			title: 'Configuration',
			templateUrl: 'partials/config.html',
			controller: 'ConfigCtrl',
		}).
		when('/action', {
			title: 'Action',
			templateUrl: 'partials/action.html',
			controller: 'ActionCtrl',
		}).
		otherwise({
			redirectTo: '/',
		});
}]);

interface IRootScope extends ng.IScope {
	title: string;
}

tsafApp.run(['$location', '$rootScope', function($location: ng.ILocationService, $rootScope: IRootScope) {
	$rootScope.$on('$routeChangeSuccess', function(event, current, previous) {
		$rootScope.title = current.$$route.title;
	});
}]);

var tsafControllers = angular.module('tsafControllers', []);

interface ITsafScope extends ng.IScope {
	active: (v: string) => any;
	json: (v: any) => string;
	btoa: (v: any) => string;
	encode: (v: string) => string;
	timeanddate: number[];
	schedule: any;
	req_from_m: (m: string) => Request;
	refresh: () => void;
}

tsafControllers.controller('TsafCtrl', ['$scope', '$route', '$http', function($scope: ITsafScope, $route: ng.route.IRouteService, $http: ng.IHttpService) {
	$scope.active = (v: string) => {
		if (!$route.current) {
			return null;
		}
		if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
			return { active: true };
		}
		return null;
	};
	$scope.json = (v: any) => {
		return JSON.stringify(v, null, '  ');
	};
	$scope.btoa = (v: any) => {
		return btoa(v);
	};
	$scope.encode = (v: string) => {
		return encodeURIComponent(v);
	};
	$scope.req_from_m = (m: string) => {
		var r = new Request();
		var q = new Query();
		q.metric = m;
		r.queries.push(q);
		return r;
	};
	$scope.refresh = () => {
		$http.get('/api/alerts').success(data => {
			angular.forEach(data.Status, (v, k) => {
				v.Touched = moment(v.Touched).utc();
				angular.forEach(v.History, (v, k) => {
					v.Time = moment(v.Time).utc();
				});
				v.last = v.History[v.History.length - 1];
			});
			$scope.schedule = data;
			$scope.timeanddate = data.TimeAndDate;
		});
	};
}]);

moment.defaultFormat = 'YYYY/MM/DD-HH:mm:ss';

moment.lang('en', {
	relativeTime: {
		future: "in %s",
		past: "%s-ago",
		s: "%ds",
		m: "%dm",
		mm: "%dm",
		h: "%dh",
		hh: "%dh",
		d: "%dd",
		dd: "%dd",
		M: "%dn",
		MM: "%dn",
		y: "%dy",
		yy: "%dy"
	},
});
