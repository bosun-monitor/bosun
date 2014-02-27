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
		when('/host', {
			templateUrl: 'partials/host.html',
			controller: 'HostCtrl',
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

class dp {
	x: number;
	y: number;
}

interface IQuery {
	aggregator?: string;
	metric: string;
	rate?: boolean;
	rateOptions?: RateOptions;
	tags?: TagSet;
	ds?: string;
	dstime?: string;
}

class Query {
	aggregator: string;
	metric: string;
	rate: boolean;
	rateOptions: RateOptions;
	tags: TagSet;
	downsample: string;
	ds: string;
	dstime: string;
	constructor(q?: IQuery) {
		this.aggregator = q && q.aggregator || 'sum';
		this.metric = q && q.metric || '';
		this.rate = q && q.rate || false;
		this.rateOptions = q && q.rateOptions || new RateOptions;
		this.ds = q && q.ds || '';
		this.dstime = q && q.dstime || '';
		this.tags = q && q.tags || new TagSet;
		this.setDs();
	}
	copy(qp: Query) {
		this.aggregator = qp.aggregator;
		this.metric = qp.metric;
		this.rate = qp.rate;
		this.rateOptions = qp.rateOptions;
		this.ds = qp.ds;
		this.dstime = qp.dstime;
		this.tags = qp.tags;
		this.setDs();
	}
	setDs() {
		if (this.dstime && this.ds) {
			this.downsample = this.dstime + '-' + this.ds;
		} else {
			this.downsample = '';
		}
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
	index: number;
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
	query_p: Query[];
	start: string;
	end: string;
	AddTab: () => void;
	setIndex: (i: number) => void;
}

tsafControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	var search = $location.search();
	var request = search.json ? JSON.parse(search.json) : new Request;
	$scope.index = parseInt($location.hash()) || 0;
	$scope.tagvs = [];
	$scope.sorted_tagks = [];
	$scope.query_p = request.Queries;
	$scope.start = request.start;
	$scope.end = request.end;
	$scope.AddTab = function() {
		$scope.index = $scope.query_p.length;
		$scope.query_p.push(new Query);
	};
	$scope.setIndex = function(i: number) {
		$scope.index = i;
	};
	$scope.GetTagKByMetric = function(index: number) {
		$scope.tagvs[index] = new TagV;
		if ($scope.query_p[index].metric) {
			$http.get('/api/tagk/' + $scope.query_p[index].metric)
				.success(function (data: string[]) {
					if (data instanceof Array) {
						var tags: TagSet = {};
						for (var i = 0; i < data.length; i++) {
							tags[data[i]] = $scope.query_p[index].tags[data[i]] || '';
							GetTagVs(data[i], index);
						}
						$scope.query_p[index].tags = tags;
						// Make sure host is always the first tag.
						$scope.sorted_tagks[index] = Object.keys(tags);
						$scope.sorted_tagks[index].sort((a, b) => {
							if (a == 'host') {
								return 1;
							} else if (b == 'host') {
								return -1;
							}
							return a.localeCompare(b);
						}).reverse();
					}
				})
				.error(function (error) {
					$scope.error = 'Unable to fetch metrics: ' + error;
				});
		}
	};
	if ($scope.query_p.length == 0) {
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
	function getRequest() {
		request = new Request;
		request.start = $scope.start;
		request.end = $scope.end;
		angular.forEach($scope.query_p, function (p) {
			if (!p.metric) {
				return
			}
			var q = new Query;
			q.copy(p);
			var tags = q.tags;
			q.tags = new TagSet;
			angular.forEach(tags, function (v, k) {
				if (v && k) {
					q.tags[k] = v;
				}
			});
			request.Queries.push(q);
		});
		return request;
	}
	$scope.Query = function() {
		$location.search('json', JSON.stringify(getRequest()));
		$route.reload();
	}
	request = getRequest();
	if (!request.Queries.length) {
		return;
	}
	$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(request)))
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

interface IHostScope extends ng.IScope {
	cpu: any;
	host: string;
	time: string;
	idata: any;
	fs: any;
	fsdata: any;
	fs_current: any;
	mem: any;
	mem_total: number;
	interfaces: string[];
	error: string;
	running: string;

}

tsafControllers.controller('HostCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHostScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	$scope.host = ($location.search()).host;
	$scope.time = ($location.search()).time;
	$scope.idata = [];
	$scope.fsdata = [];
	$scope.fs_current = [];
	var cpu_r = new Request();
	cpu_r.start = $scope.time;
	cpu_r.Queries = [
			new Query({
				metric: "os.cpu",
				rate: true,
				tags: {host: $scope.host},
			})];
	$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)))
		.success((data) => {
			$scope.cpu = data;
			$scope.running = '';
			$scope.error = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host)
		.success((data) => {
			$scope.interfaces = data;
			angular.forEach($scope.interfaces, function(i) {
				var net_bytes_r = new Request();
				net_bytes_r.start = $scope.time;
				net_bytes_r.Queries = [
					new Query({
						metric: "os.net.bytes",
						rate: true,
						tags: {host: $scope.host, iface: i, direction: "*"}
					})];
				$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)))
					.success((data) => {
						$scope.idata[$scope.interfaces.indexOf(i)] = {name: i, data: data};
						$scope.running = '';
						$scope.error = '';
					})
					.error((error) => {
						$scope.error = error;
						$scope.running = '';
					});
		})
			$scope.running = '';
			$scope.error = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$http.get('/api/tagv/mount/os.disk.fs.space_total?host=' + $scope.host)
		.success((data) => {
			$scope.fs = data;
			angular.forEach($scope.fs, function(i) {
				if ( i === '/dev/shm') {
					return;
				}
				var fs_r = new Request();
				fs_r.start = $scope.time;
				fs_r.Queries.push(new Query({
					metric: "os.disk.fs.space_total",
					tags: {host: $scope.host, mount: i}
				}));
				fs_r.Queries.push(new Query({
					metric: "os.disk.fs.space_used",
					tags: {host: $scope.host, mount: i}
				}));
				$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)))
					.success((data) => {
						$scope.fsdata[$scope.fs.indexOf(i)] = {name: i, data: [data[1]]};
						var total: number = Math.max.apply(null, data[0].data.map(function (i: dp) { return i.y }));
						var c_val: number = data[1].data.slice(-1)[0].y;
						var percent_used: number = c_val/total * 100;
						$scope.fs_current[$scope.fs.indexOf(i)] = {
								total: total,
								c_val: c_val,
								percent_used: percent_used};
						$scope.running = '';
						$scope.error = '';
					})
					.error((error) => {
						$scope.error = error;
						$scope.running = '';
					});
		})
			$scope.running = '';
			$scope.error = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	var mem_r = new Request();
	mem_r.start = $scope.time;
	mem_r.Queries.push(new Query({
		metric: "os.mem.total",
		tags: {host: $scope.host}
	}));
	mem_r.Queries.push(new Query({
		metric: "os.mem.used",
		tags: {host: $scope.host}
	}));
	$http.get('/api/query?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)))
		.success((data) => {
			$scope.mem_total = Math.max.apply(null, data[0].data.map(function (i: dp) { return i.y }))
			$scope.mem = [data[1]];
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
		link: (scope: ng.IScope, elem: any, attrs: any) => {
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
				var graph_options: any = {
					element: rgraph[0],
					height: rgraph.height(),
					min: 'auto',
					series: v,
					renderer: 'line',
				}
				if (attrs.max) {
					graph_options.max = attrs.max;
				}
				if (attrs.renderer) {
					graph_options.renderer = attrs.renderer;
				}
				var graph: any = new Rickshaw.Graph(graph_options);
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
				//Simulate a movemove so the hover appears on load
				var e = document.createEvent('MouseEvents');
				e.initEvent('mousemove', true, false);
				rgraph[0].children[0].dispatchEvent(e);
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

tsafApp.filter('bytes', function() {
	return function(bytes: any, precision: number) {
		if (bytes === 0) { return '0 bytes' };
		if (isNaN(parseFloat(bytes)) || !isFinite(bytes)) return '-';
		if (typeof precision === 'undefined') precision = 1;
		var units = ['bytes', 'kB', 'MB', 'GB', 'TB', 'PB'],
			number = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, Math.floor(number))).toFixed(precision) +  ' ' + units[number];
	}
});
