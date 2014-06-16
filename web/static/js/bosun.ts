/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="angular-sanitize.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="d3.d.ts" />

var bosunApp = angular.module('bosunApp', [
	'ngRoute',
	'bosunControllers',
	'mgcrea.ngStrap',
	'ngSanitize',
]);

bosunApp.config(['$routeProvider', '$locationProvider', function($routeProvider: ng.route.IRouteProvider, $locationProvider: ng.ILocationProvider) {
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
		when('/history', {
			title: 'Alert History',
			templateUrl: 'partials/history.html',
			controller: 'HistoryCtrl',
		}).
		when('/put', {
			title: 'Data Entry',
			templateUrl: 'partials/put.html',
			controller: 'PutCtrl',
		}).
		otherwise({
			redirectTo: '/',
		});
}]);

interface IRootScope extends ng.IScope {
	title: string;
}

bosunApp.run(['$location', '$rootScope', function($location: ng.ILocationService, $rootScope: IRootScope) {
	$rootScope.$on('$routeChangeSuccess', function(event, current, previous) {
		$rootScope.title = current.$$route.title;
	});
}]);

var bosunControllers = angular.module('bosunControllers', []);

interface IBosunScope extends ng.IScope {
	active: (v: string) => any;
	json: (v: any) => string;
	btoa: (v: any) => string;
	encode: (v: string) => string;
	panelClass: (v: string) => string;
	timeanddate: number[];
	schedule: any;
	req_from_m: (m: string) => Request;
	refresh: () => any;
	animate: () => any;
	stop: () => any;
}

bosunControllers.controller('BosunCtrl', ['$scope', '$route', '$http', function($scope: IBosunScope, $route: ng.route.IRouteService, $http: ng.IHttpService) {
	$scope.$on('$routeChangeSuccess', function(event, current, previous) {
		$scope.stop();
	});
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
	$scope.panelClass = (status: string) => {
		switch (status) {
			case "critical": return "panel-danger";
			case "unknown": return "panel-info";
			case "warning": return "panel-warning";
			case "normal": return "panel-success";
			default: return "panel-default";
		}
	};
	$scope.refresh = () => {
		$scope.schedule = null;
		$scope.animate();
		var p = $http.get('/api/alerts')
			.success(data => {
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
		p.finally($scope.stop);
		return p;
	};
	var sz = 30;
	var orig = 700;
	var light = '#4ba2d9';
	var dark = '#1f5296';
	var med = '#356eb6';
	var mult = sz / orig;
	var bgrad = 25 * mult;
	var circles = [
		[150, 150, dark],
		[550, 150, dark],
		[150, 550, light],
		[550, 550, light],
	];
	var svg = d3.select('#logo')
		.append('svg')
		.attr('height', sz)
		.attr('width', sz);
	svg.selectAll('rect.bg')
		.data([[0, light], [sz/2, dark]])
		.enter()
		.append('rect')
		.attr('class', 'bg')
		.attr('width', sz)
		.attr('height', sz / 2)
		.attr('rx', bgrad)
		.attr('ry', bgrad)
		.attr('fill', (d: any) => { return d[1]; })
		.attr('y', (d: any) => { return d[0]; });
	svg.selectAll('path.diamond')
		.data([150, 550])
		.enter()
		.append('path')
		.attr('d', (d: any) => {
			var s = 'M ' + d * mult + ' ' + 150 * mult;
			var w = 200 * mult;
			s += ' l ' + w + ' ' + w;
			s += ' l ' + -w + ' ' + w;
			s += ' l ' + -w + ' ' + -w + ' Z';
			return s;
		})
		.attr('fill', med)
		.attr('stroke', 'white')
		.attr('stroke-width', 15 * mult);
	svg.selectAll('rect.white')
		.data([150, 350, 550])
		.enter()
		.append('rect')
		.attr('class', 'white')
		.attr('width', .5)
		.attr('height', '100%')
		.attr('fill', 'white')
		.attr('x', (d: any) => { return d * mult; });
	svg.selectAll('circle')
		.data(circles)
		.enter()
		.append('circle')
		.attr('cx', (d: any) => { return d[0] * mult; })
		.attr('cy', (d: any) => { return d[1] * mult; })
		.attr('r', 62.5 * mult)
		.attr('fill', (d: any) => { return d[2]; })
		.attr('stroke', 'white')
		.attr('stroke-width', 25 * mult);
	var stopped = true;
	var transitionDuration = 500;
	$scope.animate = () => {
		if (!stopped) {
			return;
		}
		stopped = false;
		doAnimate();
	};
	function doAnimate() {
		if (stopped) {
			return;
		}
		d3.shuffle(circles);
		svg.selectAll('circle')
			.data(circles, (d: any, i: any) => { return i; })
			.transition()
			.ease('linear')
			.duration(transitionDuration)
			.attr('cx', (d: any) => { return d[0] * mult; })
			.attr('cy', (d: any) => { return d[1] * mult; })
			.attr('fill', (d: any) => { return d[2]; });
		setTimeout(doAnimate, transitionDuration);
	}
	$scope.stop = () => {
		stopped = true;
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
