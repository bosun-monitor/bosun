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

class TagSet {
	[tagk: string]: string;
}

class TagV {
	[tagk: string]: string[];
}

class RateOptions {
	counter: boolean;
	counterMax: string;
	resetValue: string;
}

class QueryParams {
	metric: string;
	rate: boolean;
	rateOptions: RateOptions;
	start: string;
	end: string;
	ds: string;
	dstime: string;
	aggregator: string;
	tags: TagSet;
	constructor() {
		this.rateOptions = new RateOptions;
		this.tags = new TagSet;
		this.aggregator = 'sum';
	}
}

class Query {
	aggregator: string;
	metric: string;
	rate: boolean;
	rateOptions: RateOptions;
	Downsample: string;
	Tags: TagSet;
	constructor(qp: QueryParams) {
		this.aggregator = qp.aggregator;
		this.metric = qp.metric;
		this.rate = qp.rate;
		this.rateOptions = qp.rateOptions;
		if (qp.dstime && qp.ds) {
			this.Downsample = qp.dstime + '-' + qp.ds
		}
		if (qp.tags) {
			var ts: TagSet = new TagSet;
			angular.forEach(qp.tags, function(v, k) {
				if(v) {
					ts[k] = v;
				}
			});
		}
		this.Tags = ts;
	}
}

class Request {
	start: string;
	end: string;
	Queries: Query[];
	constructor() {
		this.start = '1h-ago';
		this.Queries = [];
	}
}

interface IGraphScope extends ng.IScope {
	error: string;
	running: string;
	metrics: string[];
	tagvs: TagV[];
	tags: TagSet;
	sorted_tagks: string[][];
	query: string;
	aggregators: string[];
	dsaggregators: string[];
	GetTagKByMetric: (index: number) => void;
	Query: () => void;
	TagsAsQs: (ts: TagSet) => string;
	MakeParam: (k: string, v: string) => string;
	GetTagVs: (k: string) => void;
	result: any;
	dt: any;
	series: any;
	query_p: QueryParams[];
	request: any;
	start: string;
	end: string;
	tabs: boolean[];
	AddTab: () => void;
}

tsafControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	var search = $location.search();
	$scope.tagvs = [];
	$scope.sorted_tagks = [];
	$scope.tabs = [];
	$scope.query_p = [];
	$scope.request = search.json ? JSON.parse( search.json ) : new Request;
	$scope.start = $scope.request.start;
	$scope.end = $scope.request.end;
	$scope.AddTab = function() {
		$scope.query_p.push(new QueryParams);
		$scope.tabs.push(true)
	}
	$scope.GetTagKByMetric = function(index: number) {
		var tags: TagSet = {};
		$scope.tagvs[index] = new TagV;
		if ($scope.query_p[index].metric) {
			$http.get('/api/tagk/' + $scope.query_p[index].metric)
				.success(function (data: string[]) {
					if (data instanceof Array) {
						for (var i = 0; i < data.length; i++) {
							tags[data[i]] = $scope.query_p[index].tags[data[i]] || '';
							GetTagVs(data[i], index);
						}
						$scope.query_p[index].tags = tags;
						//Make sure Host is always the first tag
						$scope.sorted_tagks[index] = Object.keys(tags);
						var hosti: number = $scope.sorted_tagks[index].indexOf("host");
						if (hosti > 0) {
							$scope.sorted_tagks[index].move(hosti, 0);
						}
					}
				})
				.error(function (error) {
					$scope.error = 'Unable to fetch metrics: ' + error;
				});
		}
	}
	var j = 0;
	angular.forEach($scope.request.Queries, function(q) {
		$scope.query_p.push(new QueryParams);
		$scope.tabs.push(true);
		$scope.query_p[j].metric = q.metric;
		$scope.query_p[j].ds = q.ds;
		$scope.query_p[j].dstime = q.dstime;
		$scope.query_p[j].aggregator = q.aggregator || 'sum';
		$scope.query_p[j].rate = q.rate == true;
		if (q.RateOptions) {
			$scope.query_p[j].rateOptions.counter = q.rateOptions.counter == true;
			$scope.query_p[j].rateOptions.counterMax = q.rateOptions.counterMax;
			$scope.query_p[j].rateOptions.resetValue = q.rateOptions.resetValue;
		}
		$scope.query_p[j].tags = q.Tags || new TagSet;
		$scope.GetTagKByMetric(j)
		j += 1
	})
	if (j == 0) {
		$scope.AddTab();
	}
	$http.get('/api/metric')
		.success(function (data: string[]) {
			$scope.metrics = data;
		})
		.error(function (error) {
			$scope.error = 'Unable to fetch metrics: ' + error;
		});

	function GetTagVs(k: string, index: number) {
		$http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric)
			.success(function (data: string[]) {
				$scope.tagvs[index][k] = data;
			})
			.error(function (error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	$scope.Query = function() {
		$scope.request = new Request;
		$scope.request.start = $scope.start;
		$scope.request.end = $scope.end;
		angular.forEach($scope.query_p, function (p) {
			if (p.metric) {
				var query = new Query(p);
				$scope.request.Queries.push(query);
			}
		});
		$location.search('json', JSON.stringify($scope.request));
		$route.reload();
	}
	if ($scope.request.Queries.length < 1) {
		return;
	}
	$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify($scope.request)))
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
				graph.render();
				var legend = angular.element('.rlegend', elem)[0];
				var Hover = Rickshaw.Class.create(Rickshaw.Graph.HoverDetail, {
					render: function(args: any) {
						legend.innerHTML = args.formattedXValue;
						args.detail.
							sort((a: any, b: any) => { return a.order - b.order }).
							forEach(function(d: any) {
								var line = document.createElement('div');
								line.className = 'rline';
								var swatch = document.createElement('div');
								swatch.className = 'rswatch';
								swatch.style.backgroundColor = d.series.color;
								var label = document.createElement('div');
								label.className = 'rlabel';
								label.innerHTML = d.name + ": " + d.formattedYValue;
								line.appendChild(swatch);
								line.appendChild(label);
								legend.appendChild(line);
								var dot = document.createElement('div');
								dot.className = 'dot';
								dot.style.top = graph.y(d.value.y0 + d.value.y) + 'px';
								dot.style.borderColor = d.series.color;
								this.element.appendChild(dot);
								dot.className = 'dot active';
								this.show();
							}, this);
					}
				});
				var hover = new Hover({graph: graph});
			});
		},
	};
});

tsafApp.directive("tooltip", function() {
	return {
		link: function(scope: IGraphScope, elem: any, attrs: any) {
			angular.element(elem[0]).tooltip({placement: "bottom"});
		},
	};
});

tsafApp.directive('showtab', function () {
	return {
		link: function (scope: ng.IScope, elem: any, attrs: any) {
			elem.click(function(e: any) {
				e.preventDefault();
				$(elem).tab('show');
			});
		}
	};
});

//Extras
interface Array {
	move(old_index: number, new_index: number): string[];
}

Array.prototype.move = function (old_index, new_index) {
	if (new_index >= this.length) {
		var k = new_index - this.length;
		while ((k--) + 1) {
			this.push(undefined);
		}
	}
	this.splice(new_index, 0, this.splice(old_index, 1)[0]);
	return this; // for testing purposes
};