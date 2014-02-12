/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="bootstrap.d.ts" />
/// <reference path="moment.d.ts" />
/// <reference path="rickshaw.d.ts" />

var tsafApp = angular.module('tsafApp', [
	'ngRoute',
	'tsafControllers',
	'mgcrea.ngStrap',
]);

tsafApp.config(['$routeProvider', '$locationProvider', function($routeProvider: ng.route.IRouteProvider, $locationProvider: ng.ILocationProvider) {
	$locationProvider.html5Mode(true);
	$routeProvider.
		when('/', {
			templateUrl: 'partials/dashboard.html',
			controller: 'DashboardCtrl',
		}).
		when('/items', {
			templateUrl: 'partials/items.html',
			controller: 'ItemsCtrl',
		}).
		when('/expr', {
			templateUrl: 'partials/expr.html',
			controller: 'ExprCtrl',
		}).
		when('/graph', {
			templateUrl: 'partials/graph.html',
			controller: 'GraphCtrl',
		}).
		otherwise({
			redirectTo: '/',
		});
}]);

var tsafControllers = angular.module('tsafControllers', []);

interface ITsafScope extends ng.IScope {
	active: (v: string) => any;
}

tsafControllers.controller('TsafCtrl', ['$scope', '$route', function($scope: ITsafScope, $route: ng.route.IRouteService) {
	$scope.active = (v: string) => {
		if (!$route.current) {
			return null;
		}
		if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
			return {active: true};
		}
		return null;
	};
}]);

interface IDashboardScope extends ng.IScope {
	schedule: any;
	last: any;
	collapse: (i: number) => void;
	panel: (status: string) => string;
}

tsafControllers.controller('DashboardCtrl', ['$scope', '$http', function($scope: IDashboardScope, $http: ng.IHttpService) {
	$http.get('/api/alerts').success(data => {
		angular.forEach(data.Status, (v, k) => {
			v.Touched = moment(v.Touched).utc();
			angular.forEach(v.History, (v, k) => {
				v.Time = moment(v.Time).utc();
			});
			v.last = v.History[v.History.length-1];
		});
		$scope.schedule = data;
	});
	$scope.collapse = (i: number) => {
		$('#collapse' + i).collapse('toggle');
	};
	$scope.panel = (status: string) => {
		if (status == "critical") {
			return "panel-danger";
		} else if (status == "warning") {
			return "panel-warning";
		}
		return "panel-default";
	};
}]);

interface IItemsScope extends ng.IScope {
	metrics: string[];
	hosts: string[];
	status: string;
}

tsafControllers.controller('ItemsCtrl', ['$scope', '$http', function($scope: IItemsScope, $http: ng.IHttpService){
	$http.get('/api/metric')
		.success(function (data: string[]) {
			$scope.metrics = data;
		})
		.error(function (error) {
			$scope.status = 'Unable to fetch metrics: ' + error;
		});
	$http.get('/api/tagv/host')
		.success(function (data: string[]) {
			$scope.hosts = data;
		})
		.error(function (error) {
			$scope.status = 'Unable to fetch hosts: ' + error;
		});
}]);

interface IExprScope extends ng.IScope {
	expr: string;
	error: string;
	running: string;
	result: any;
	set: () => void;
	json: (v: any) => string;
}

tsafControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function($scope: IExprScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var current: string = $location.hash();
	if (!current) {
		$location.hash('avg(q("avg:os.cpu{host=ny-devtsdb04.ds.stackexchange.com}", "5m")) > 0.5');
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	$http.get('/api/expr?q=' + encodeURIComponent(current))
		.success((data) => {
			$scope.result = data;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.json = (v: any) => {
		return JSON.stringify(v, null, '  ');
	};
	$scope.set = () => {
		$location.hash($scope.expr);
		$route.reload();
	};
}]);

interface TagSet {
	[tagk: string]: string;
}

interface TagV {
	[tagk: string]: string[];
}

interface IGraphScope extends ng.IScope {
	error: string;
	running: string;
	metric: string;
	ds: string;
	dstime: string;
	metrics: string[];
	tagvs: TagV;
	tagset: TagSet;
	query: string;
	rate: boolean;
	counter: boolean;
	cmax: string;
	creset: string;
	start: string;
	end: string;
	aggregators: string[];
	dsaggregators: string[];
	aggregator: string;
	GetTagKByMetric: () => void;
	Query: () => void;
	TagsAsQs: (ts: TagSet) => string;
	MakeParam: (k: string, v: string) => string;
	GetTagVs: (k: string) => void;
	result: any;
	dt: any;
	series: any;
	height: number;
}

tsafControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	var search = $location.search();
	$scope.ds = search.ds || '';
	$scope.aggregator = search.aggregator || 'sum';
	$scope.rate = search.rate == 'true';
	$scope.start = search.start || '1d-ago';
	$scope.metric = search.metric;
	$scope.counter = search.counter == 'true';
	$scope.dstime = search.dstime;
	$scope.end = search.end;
	$scope.cmax = search.cmax;
	$scope.creset = search.creset;
	$scope.tagset = search.tags ? JSON.parse(search.tags) : {};
	$http.get('/api/metric')
		.success(function (data: string[]) {
			$scope.metrics = data;
		})
		.error(function (error) {
			$scope.error = 'Unable to fetch metrics: ' + error;
		});
	$scope.GetTagKByMetric = function() {
		var tagset: TagSet = {};
		$scope.tagvs = {};
		$http.get('/api/tagk/' + $scope.metric)
			.success(function (data: string[]) {
				if (data instanceof Array) {
					for (var i = 0; i < data.length; i++) {
						tagset[data[i]] = $scope.tagset[data[i]] || '';
						GetTagVs(data[i]);
					}
					$scope.tagset = tagset;
				}
			})
			.error(function (error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	function TagsAsQS(ts: TagSet) {
		var qts = new Array<string>();
		for (var key in $scope.tagset) {
			if ($scope.tagset.hasOwnProperty(key)) {
				if ($scope.tagset[key] != "") {
					qts.push(key);
					qts.push($scope.tagset[key])
				}
			}
		}
		return qts.join();
	}
	function MakeParam(qs: string[], k: string, v: string) {
		if (v) {
			qs.push(encodeURIComponent(k) + "=" + encodeURIComponent(v));
		}
	}
	function GetTagVs(k: string) {
		$http.get('/api/tagv/' + k + '/' + $scope.metric)
			.success(function (data: string[]) {
				$scope.tagvs[k] = data;
			})
			.error(function (error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	$scope.Query = function() {
		$location.search('start', $scope.start || null);
		$location.search('end', $scope.end || null);
		$location.search('aggregator', $scope.aggregator);
		$location.search('metric', $scope.metric);
		$location.search('rate', $scope.rate.toString());
		$location.search('ds', $scope.ds || null);
		$location.search('dstime', $scope.dstime || null);
		$location.search('counter', $scope.counter.toString());
		$location.search('cmax', $scope.cmax || null);
		$location.search('creset', $scope.creset || null);
		$location.search('tags', JSON.stringify($scope.tagset));
		$route.reload();
	};
	if (!$scope.metric) {
		return;
	}
	var qs: string[] = [];
	MakeParam(qs, "start", $scope.start);
	MakeParam(qs, "end", $scope.end);
	MakeParam(qs, "aggregator", $scope.aggregator);
	MakeParam(qs, "metric", $scope.metric);
	MakeParam(qs, "rate", $scope.rate.toString());
	MakeParam(qs, "tags", TagsAsQS($scope.tagset));
	if ($scope.ds && $scope.dstime) {
		MakeParam(qs, "downsample", $scope.dstime + '-' + $scope.ds);
	}
	MakeParam(qs, "counter", $scope.counter.toString());
	MakeParam(qs, "cmax", $scope.cmax);
	MakeParam(qs, "creset", $scope.creset);
	$scope.query = qs.join('&');
	$scope.running = $scope.query;
	$http.get('/api/query?' + $scope.query)
		.success((data) => {
			$scope.result = data;
			$scope.running = '';
			$scope.error = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
}]);

tsafApp.directive("tsRickshaw", function() {
	return {
		templateUrl: '/partials/rickshaw.html',
		link: (scope: IGraphScope, elem: any, attrs: any) => {
			scope.$watch(attrs.tsRickshaw, function(v: any) {
				if (!v) {
					return;
				}
				var palette: any = new Rickshaw.Color.Palette();
				angular.forEach(v, function(i) {
					if (!i.color) {
						i.color = palette.color();
					}
				});
				var rgraph = angular.element('.rgraph', elem);
				var graph: any = new Rickshaw.Graph({
					element: rgraph[0],
					height: rgraph.height(),
					min: 'auto',
					series: v,
					renderer: 'line',
				});
				var x_axis: any = new Rickshaw.Graph.Axis.Time({
					graph: graph,
					timeFixture: new Rickshaw.Fixtures.Time(),
				});
				var y_axis: any = new Rickshaw.Graph.Axis.Y({
					graph: graph,
					orientation: 'left',
					tickFormat: Rickshaw.Fixtures.Number.formatKMBT,
					element: angular.element('.y_axis', elem)[0],
				});
				var hoverDetail: any = new Rickshaw.Graph.HoverDetail( {
					graph: graph,
				});
				var legend: any = new Rickshaw.Graph.Legend( {
					graph: graph,
					element: angular.element('.legend', elem)[0],
				});
				graph.render();
			});
		},
	};
});
