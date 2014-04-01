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
		when('/egraph', {
			templateUrl: 'partials/egraph.html',
			controller: 'EGraphCtrl',
		}).
		when('/graph', {
			templateUrl: 'partials/graph.html',
			controller: 'GraphCtrl',
		}).
		when('/host', {
			templateUrl: 'partials/host.html',
			controller: 'HostCtrl',
		}).
		when('/rule', {
			templateUrl: 'partials/rule.html',
			controller: 'RuleCtrl',
		}).
		when('/silence', {
			templateUrl: 'partials/silence.html',
			controller: 'SilenceCtrl',
		}).
		otherwise({
			redirectTo: '/',
		});
}]);

var tsafControllers = angular.module('tsafControllers', []);

interface ITsafScope extends ng.IScope {
	active: (v: string) => any;
	json: (v: any) => string;
	btoa: (v: any) => string;
	zws: (v: string) => string; // adds the unicode zero-width space character where appropriate
	time: (v: any) => string; // formats a timestamp
	timeanddate: number[];
	schedule: any;
}

tsafControllers.controller('TsafCtrl', ['$scope', '$route', '$http', function($scope: ITsafScope, $route: ng.route.IRouteService, $http: ng.IHttpService) {
	$scope.active = (v: string) => {
		if (!$route.current) {
			return null;
		}
		if ($route.current.loadedTemplateUrl == 'partials/' + v + '.html') {
			return {active: true};
		}
		return null;
	};
	$scope.json = (v: any) => {
		return JSON.stringify(v, null, '  ');
	};
	$scope.btoa = (v: any) => {
		return btoa(v);
	};
	$scope.zws = (v: string) => {
		return v.replace(/([,{}()])/g, '$1\u200b');
	};
	$http.get('/api/alerts').success(data => {
		angular.forEach(data.Status, (v, k) => {
			v.Touched = moment(v.Touched).utc();
			angular.forEach(v.History, (v, k) => {
				v.Time = moment(v.Time).utc();
			});
			v.last = v.History[v.History.length-1];
		});
		$scope.schedule = data;
		$scope.timeanddate = data.TimeAndDate;
	});
}]);

interface IDashboardScope extends ITsafScope {
	last: any;
	collapse: (i: number) => void;
	panelClass: (status: number) => string;
	statusString: (status: number) => string;
}

tsafControllers.controller('DashboardCtrl', ['$scope', function($scope: IDashboardScope) {
	$scope.collapse = (i: number) => {
		$('#collapse' + i).collapse('toggle');
	};
	$scope.panelClass = (status: number) => {
		switch (status) {
		case 3: return "panel-danger"; break;
		case 2: return "panel-warning"; break;
		default: return "panel-default"; break;
		}
	};
	$scope.statusString = (status: number) => {
		switch (status) {
		case 3: return "critical"; break;
		case 2: return "warning"; break;
		case 1: return "normal"; break;
		default: return "unknown"; break;
		}
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
	queries: any;
	set: () => void;
}

tsafControllers.controller('ExprCtrl', ['$scope', '$http', '$location', '$route', function($scope: IExprScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var current = $location.hash();
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		$location.hash(btoa('avg(q("avg:rate:os.cpu{host=ny-devtsdb04}", "5m", "")) > 80'));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	$http.get('/api/expr?q=' + encodeURIComponent(current))
		.success((data) => {
			$scope.result = data.Results;
			$scope.queries = data.Queries;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.set = () => {
		$location.hash(btoa($scope.expr));
		$route.reload();
	};
}]);

interface IEGraphScope extends ng.IScope {
	expr: string;
	error: string;
	running: string;
	result: any;
	render: string;
	renderers: string[];
	bytes: boolean;
	set: () => void;
}

tsafControllers.controller('EGraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IEGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var search = $location.search();
	var current = search.q;
	try {
		current = atob(current);
	}
	catch (e) {}
	$scope.bytes = search.bytes;
	$scope.bytes = !!$scope.bytes;
	$scope.renderers = ['area', 'bar', 'line', 'scatterplot'];
	$scope.render = search.render || 'scatterplot';
	if (!current) {
		$location.search('q', btoa('q("avg:rate:os.cpu{host=ny-devtsdb04}", "5m", "")'));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	var width: number = $('.chart').width()
	$http.get('/api/egraph?q=' + encodeURIComponent(current) + '&autods=' + width)
		.success((data) => {
			$scope.result = data;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.set = () => {
		$location.search('q', btoa($scope.expr));
		$location.search('render', $scope.render);
		$location.search('bytes', $scope.bytes);
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
	counterMax: number;
	resetValue: number;
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
	constructor(q?: any) {
		this.aggregator = q && q.aggregator || 'sum';
		this.metric = q && q.metric || '';
		this.rate = q && q.rate || false;
		this.rateOptions = q && q.rateOptions || new RateOptions;
		this.ds = q && q.ds || '';
		this.dstime = q && q.dstime || '';
		this.tags = q && q.tags || new TagSet;
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
	queries: Query[];
	constructor() {
		this.start = '1h-ago';
		this.queries = [];
	}
	prune() {
		for(var i = 0; i < this.queries.length; i++) {
			angular.forEach(this.queries[i], (v, k) => {
				switch (typeof v) {
				case "string":
					if (!v) {
						delete this.queries[i][k];
					}
					break;
				case "boolean":
					if (!v) {
						delete this.queries[i][k];
					}
					break;
				case "object":
					if (Object.keys(v).length == 0) {
						delete this.queries[i][k];
					}
					break;
				}
			});
		}
	}
}

interface IGraphScope extends ng.IScope {
	index: number;
	url: string;
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
	autods: boolean;
}

tsafControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', function($scope: IGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	var search = $location.search();
	var j = search.json;
	if (search.b64) {
		j = atob(search.b64);
	}
	var request = j ? JSON.parse(j) : new Request;
	$scope.index = parseInt($location.hash()) || 0;
	$scope.tagvs = [];
	$scope.sorted_tagks = [];
	$scope.query_p = request.queries;
	$scope.start = request.start;
	$scope.end = request.end;
	$scope.autods = search.autods;
	if (typeof $scope.autods == 'undefined') {
		$scope.autods = true;
	}
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
				return;
			}
			var q = new Query(p);
			var tags = q.tags;
			q.tags = new TagSet;
			angular.forEach(tags, function (v, k) {
				if (v && k) {
					q.tags[k] = v;
				}
			});
			request.queries.push(q);
		});
		return request;
	}
	$scope.Query = function() {
		$location.search('b64', btoa(JSON.stringify(getRequest())));
		$location.search('autods', $scope.autods);
		$route.reload();
	}
	request = getRequest();
	if (!request.queries.length) {
		return;
	}
	var autods = $scope.autods ? autods = '&autods=' + $('.chart').width() : '';
	request.prune();
	$http.get('/api/graph?' + 'b64=' + btoa(JSON.stringify(request)) + autods)
		.success((data) => {
			$scope.result = data;
			$scope.url = $location.absUrl();
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
	cpu_r.queries = [
		new Query({
			metric: "os.cpu",
			rate: true,
			tags: {host: $scope.host},
		})
	];
	var width: number = $('.chart').width();
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + '&autods=' + width)
		.success((data) => {
			data[0].name = 'Percent Used';
			$scope.cpu = data;
		});

	$http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host)
		.success((data) => {
			$scope.interfaces = data;
			angular.forEach($scope.interfaces, function(i) {
				var net_bytes_r = new Request();
				net_bytes_r.start = $scope.time;
				net_bytes_r.queries = [
					new Query({
						metric: "os.net.bytes",
						rate: true,
						tags: {host: $scope.host, iface: i, direction: "*"},
					})
				];
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + '&autods=' + width)
					.success((data) => {
						angular.forEach(data, function(d) {
							d.data = d.data.map((dp: any) => { return {x: dp.x, y: dp.y*8}});
							if (d.name.indexOf("direction=out") != -1) {
								d.data = d.data.map((dp: any) => { return {x: dp.x, y: dp.y*-1}});
								d.name = "out";
							} else {
								d.name = "in";
							}
						});
						$scope.idata[$scope.interfaces.indexOf(i)] = {name: i, data: data};
					});
			});
		});
	$http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host)
		.success((data) => {
			$scope.fs = data;
			angular.forEach($scope.fs, function(i) {
				if (i == '/dev/shm') {
					return;
				}
				var fs_r = new Request();
				fs_r.start = $scope.time;
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_total",
					tags: {host: $scope.host, disk: i},
				}));
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_used",
					tags: {host: $scope.host, disk: i},
				}));
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + '&autods=' + width)
					.success((data) => {
						data[1].name = "Used";
						$scope.fsdata[$scope.fs.indexOf(i)] = {name: i, data: [data[1]]};
						var total: number = Math.max.apply(null, data[0].data.map((d: any) => { return d.y; }));
						var c_val: number = data[1].data.slice(-1)[0].y;
						var percent_used: number = c_val/total * 100;
						$scope.fs_current[$scope.fs.indexOf(i)] = {
							total: total,
							c_val: c_val,
							percent_used: percent_used,
						};
					});
			});
		});
	var mem_r = new Request();
	mem_r.start = $scope.time;
	mem_r.queries.push(new Query({
		metric: "os.mem.total",
		tags: {host: $scope.host},
	}));
	mem_r.queries.push(new Query({
		metric: "os.mem.used",
		tags: {host: $scope.host},
	}));
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + '&autods=' + width)
		.success((data) => {
			data[1].name = "Used";
			$scope.mem_total = Math.max.apply(null, data[0].data.map((d: any) => { return d.y; }));
			$scope.mem = [data[1]];
		});
}]);

interface IRuleScope extends IExprScope {
	date: string;
	time: string;
	shiftEnter: ($event: any) => void;
}

tsafControllers.controller('RuleCtrl', ['$scope', '$http', '$location', '$route', function($scope: IRuleScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService){
	var search = $location.search();
	var current = search.rule;
	$scope.date = search.date || '';
	$scope.time = search.time || '';
	try {
		current = atob(current);
	}
	catch (e) {
		current = '';
	}
	if (!current) {
		var def = '$t = "5m"\n' +
			'crit = avg(q("avg:os.cpu", $t, "")) > 10';
		$location.search('rule', btoa(def));
		return;
	}
	$scope.expr = current;
	$scope.running = current;
	$http.get('/api/rule?' +
		'rule=' + encodeURIComponent(current) +
		'&date=' + encodeURIComponent($scope.date) +
		'&time=' + encodeURIComponent($scope.time))
		.success((data) => {
			$scope.result = data.Results;
			$scope.queries = data.Queries;
			$scope.running = '';
		})
		.error((error) => {
			$scope.error = error;
			$scope.running = '';
		});
	$scope.shiftEnter = function($event: any) {
		if ($event.keyCode == 13 && $event.shiftKey) {
			$scope.set();
		}
	}
	$scope.set = () => {
		$location.search('rule', btoa($scope.expr));
		$location.search('date', $scope.date || null);
		$location.search('time', $scope.time || null);
		$route.reload();
	};
}]);

interface ISilenceScope extends IExprScope {
	silences: any;
	error: string;
	start: string;
	end: string;
	duration: string;
	alert: string;
	hosts: string;
	tags: string;
	testSilences: any;
	test: () => void;
	confirm: () => void;
	clear: (id: string) => void;
}

tsafControllers.controller('SilenceCtrl', ['$scope', '$http', function($scope: ISilenceScope, $http: ng.IHttpService){
	$scope.duration = '1h';
	function get() {
		$http.get('/api/silence/get')
			.success((data) => {
				$scope.silences = data;
			})
			.error((error) => {
				$scope.error = error;
			});
	}
	get();
	function getData() {
		var tags = $scope.tags.split(',');
		if ($scope.hosts) {
			tags.push('host=' + $scope.hosts.split(/[ ,|]+/).join('|'));
		}
		tags = tags.filter((v) => { return v != ""; });
		var data: any = {
			start: $scope.start,
			end: $scope.end,
			duration: $scope.duration,
			alert: $scope.alert,
			tags: tags.join(','),
		};
		return data;
	}
	$scope.test = () => {
		$scope.error = null;
		$http.post('/api/silence/set', getData())
			.success((data) => {
				if (data.length == 0) {
					data = [{Name: '(none)'}];
				}
				$scope.testSilences = data;
			})
			.error((error) => {
				$scope.error = error;
			});
	};
	$scope.confirm = () => {
		$scope.error = null;
		$scope.testSilences = null;
		var data = getData();
		data.confirm = "true";
		$http.post('/api/silence/set', data)
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				$scope.testSilences = null;
				get();
			});
	};
	$scope.clear = (id: string) => {
		$scope.error = null;
		$http.post('/api/silence/clear', {id: id})
			.error((error) => {
				$scope.error = error;
			})
			.finally(() => {
				get();
			});
	};
}]);

tsafApp.directive('tsResults', function() {
	return {
		templateUrl: '/partials/results.html',
	};
});

tsafApp.directive("tsTime", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTime, (v: any) => {
				var m = moment(v).utc();
				var el = document.createElement('a');
				el.innerText = m.format('YYYY-MM-DD HH:MM:SS ZZ') +
					' (' +
					m.fromNow() +
					')';
				el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
				el.href += m.format('YYYYMMDDTHHMM');
				el.href += '&p1=0';
				angular.forEach(scope.timeanddate, (v, k) => {
					el.href += '&p' + (k + 2) + '=' + v;
				});
				elem.html(el);
			});
		},
	};
});

tsafApp.directive("tsRickshaw", ['$filter', function($filter: ng.IFilterService) {
	return {
		templateUrl: '/partials/rickshaw.html',
		link: (scope: ng.IScope, elem: any, attrs: any) => {
			scope.$watch(attrs.tsRickshaw, function(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
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
					interpolation: 'linear',
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
					tickFormat: function(y: any) {
						var o: any = d3.formatPrefix(y)
						// The precision arg to d3.formatPrefix seems broken, so using round
						// http://stackoverflow.com/questions/10310613/variable-precision-in-d3-format
						return d3.round(o.scale(y),2) + o.symbol;
						},
					element: angular.element('.y_axis', elem)[0],
				});
				if (attrs.bytes == "true") {
					y_axis.tickFormat = function(y: any) {
						return $filter('bytes')(y);
						}
				}
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
								if (attrs.bytes == "true") {
									label.innerHTML = d.name + ": " + $filter('bytes')(d.formattedYValue);
								}
								if (attrs.bits) {
									label.innerHTML = d.name + ": " + $filter('bits')(d.formattedYValue);
								}
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
}]);

tsafApp.directive("tooltip", function() {
	return {
		link: function(scope: IGraphScope, elem: any, attrs: any) {
			angular.element(elem[0]).tooltip({placement: "bottom"});
		},
	};
});

tsafApp.filter('bytes', function() {
	return function(bytes: any, precision: number) {
		if (!bytes) { return '0 B' };
		if (isNaN(parseFloat(bytes)) || !isFinite(bytes)) return '-';
		if (typeof precision == 'undefined') precision = 1;
		var units = ['B', 'kB', 'MB', 'GB', 'TB', 'PB'],
			number = Math.floor(Math.log(bytes) / Math.log(1024));
		return (bytes / Math.pow(1024, Math.floor(number))).toFixed(precision) +  ' ' + units[number];
	}
});

tsafApp.filter('bits', function() {
	return function(b: any, precision: number) {
		if (!b) { return '0 b' };
		if (b < 0) {
			b = -b;
		}
		if (isNaN(parseFloat(b)) || !isFinite(b)) return '-';
		if (typeof precision == 'undefined') precision = 1;
		var units = ['b', 'kb', 'Mb', 'Gb', 'Tb', 'Pb'],
			number = Math.floor(Math.log(b) / Math.log(1024));
		return (b / Math.pow(1024, Math.floor(number))).toFixed(precision) +  ' ' + units[number];
	}
});

//This is modeled after the linky function, but drops support for sanitize so we don't have to
//import an unminified angular-sanitize module
tsafApp.filter('linkq',  ['$sanitize', function($sanitize: ng.sanitize.ISanitizeService) {
	var QUERY_REGEXP: RegExp = /((q|band)\([^)]+\))/;
	return function(text: string, target: string) {
		if (!text) return text;
		var raw = text;
		var html: string[] = [];
		var url: string;
		var i: number;
		var match: any;
		while ((match = raw.match(QUERY_REGEXP))) {
			url = '/egraph?q=' + btoa(match[0]);
			i = match.index;
			addText(raw.substr(0, i));
			addLink(url, match[0]);
			raw = raw.substring(i + match[0].length);
		}
		addText(raw);
		return $sanitize(html.join(''));
		function addText(text: string) {
			if (!text) {
				return;
			}
			var el = document.createElement('div');
			el.innerText = el.textContent = text;
			html.push(el.innerHTML);
		}
		function addLink(url: string, text: string) {
			html.push('<a ');
			if (angular.isDefined(target)) {
				html.push('target="');
				html.push(target);
				html.push('" ');
			}
			html.push('href="');
			html.push(url);
			html.push('">');
			addText(text);
			html.push('</a>');
		}
	};
}]);