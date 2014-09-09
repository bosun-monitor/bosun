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
	derivative: string;
	constructor(q?: any) {
		this.aggregator = q && q.aggregator || 'sum';
		this.metric = q && q.metric || '';
		this.rate = q && q.rate || false;
		this.rateOptions = q && q.rateOptions || new RateOptions;
		if (q && !q.derivative) {
			// back compute derivative from q
			if (!this.rate) {
				this.derivative = 'gauge';
			} else if (this.rateOptions.counter) {
				this.derivative = 'counter';
			} else {
				this.derivative = 'rate';
			}
		} else {
			this.derivative = q && q.derivative || 'auto';
		}
		this.ds = q && q.ds || '';
		this.dstime = q && q.dstime || '';
		this.tags = q && q.tags || new TagSet;
		this.setDs();
		this.setDerivative();
	}
	setDs() {
		if (this.dstime && this.ds) {
			this.downsample = this.dstime + '-' + this.ds;
		} else {
			this.downsample = '';
		}
	}
	setDerivative() {
		var max = this.rateOptions.counterMax;
		this.rate = false;
		this.rateOptions = new RateOptions();
		switch (this.derivative) {
			case "rate":
				this.rate = true;
				break;
			case "counter":
				this.rate = true;
				this.rateOptions.counter = true;
				this.rateOptions.counterMax = max;
				this.rateOptions.resetValue = 1;
				break;
			case "gauge":
				this.rate = false;
				break;
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
		for (var i = 0; i < this.queries.length; i++) {
			angular.forEach(this.queries[i], (v, k) => {
				var qi: any = this.queries[i];
				switch (typeof v) {
					case "string":
						if (!v) {
							delete qi[k];
						}
						break;
					case "boolean":
						if (!v) {
							delete qi[k];
						}
						break;
					case "object":
						if (Object.keys(v).length == 0) {
							delete qi[k];
						}
						break;
				}
			});
		}
	}
}

var graphRefresh: any;

interface IGraphScope extends ng.IScope {
	index: number;
	url: string;
	error: string;
	running: string;
	warning: string;
	metrics: string[];
	tagvs: TagV[];
	tags: TagSet;
	sorted_tagks: string[][];
	query: string;
	aggregators: string[];
	rate_options: string[];
	dsaggregators: string[];
	GetTagKByMetric: (index: number) => void;
	Query: () => void;
	TagsAsQs: (ts: TagSet) => string;
	MakeParam: (k: string, v: string) => string;
	GetTagVs: (k: string) => void;
	result: any;
	queries: string[];
	dt: any;
	series: any;
	query_p: Query[];
	start: string;
	end: string;
	AddTab: () => void;
	setIndex: (i: number) => void;
	autods: boolean;
	refresh: boolean;
	SwitchTimes: () => void;
	duration_map: any;
	animate: () => any;
	stop: () => any;
}

bosunControllers.controller('GraphCtrl', ['$scope', '$http', '$location', '$route', '$timeout', function($scope: IGraphScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $timeout: ng.ITimeoutService) {
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.rate_options = ["auto", "gauge", "counter", "rate"];
	var search = $location.search();
	var j = search.json;
	if (search.b64) {
		j = atob(search.b64);
	}
	var request = j ? JSON.parse(j) : new Request;
	$scope.index = parseInt($location.hash()) || 0;
	$scope.tagvs = [];
	$scope.sorted_tagks = [];
	$scope.query_p = [];
	angular.forEach(request.queries, (q, i) => {
		$scope.query_p[i] = new Query(q);
	});
	$scope.start = request.start;
	$scope.end = request.end;
	$scope.autods = search.autods != 'false';
	$scope.refresh = search.refresh == 'true';
	var duration_map: any = {
		"s": "s",
		"m": "m",
		"h": "h",
		"d": "d",
		"w": "w",
		"n": "M",
		"y": "y",
	};
	var isRel = /^(\d+)(\w)-ago$/;
	function RelToAbs(m: RegExpExecArray) {
		return moment().utc().subtract(parseFloat(m[1]), duration_map[m[2]]).format();
	}
	function AbsToRel(s: string) {
		//Not strict parsing of the time format. For example, just "2014" will be valid
		var t = moment.utc(s, moment.defaultFormat).fromNow();
		return t;
	}
	function SwapTime(s: string) {
		if (!s) {
			return moment().utc().format();
		}
		var m = isRel.exec(s);
		if (m) {
			return RelToAbs(m);
		}
		return AbsToRel(s);
	}
	$scope.SwitchTimes = function() {
		$scope.start = SwapTime($scope.start);
		$scope.end = SwapTime($scope.end);
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
				.success(function(data: string[]) {
					if (!angular.isArray(data)) {
						return;
					}
					var tags = $scope.query_p[index].tags || {};
					for (var i = 0; i < data.length; i++) {
						var d = data[i];
						if ($scope.query_p[index].tags) {
							tags[d] = $scope.query_p[index].tags[d];
						}
						if (!tags[d]) {
							tags[d] = '';
						}
						GetTagVs(d, index);
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
				})
				.error(function(error) {
					$scope.error = 'Unable to fetch metrics: ' + error;
				});
		}
	};
	if ($scope.query_p.length == 0) {
		$scope.AddTab();
	}
	$http.get('/api/metric')
		.success(function(data: string[]) {
			$scope.metrics = data;
		})
		.error(function(error) {
			$scope.error = 'Unable to fetch metrics: ' + error;
		});

	function GetTagVs(k: string, index: number) {
		$http.get('/api/tagv/' + k + '/' + $scope.query_p[index].metric)
			.success(function(data: string[]) {
				data.sort();
				$scope.tagvs[index][k] = data;
			})
			.error(function(error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	function getRequest() {
		request = new Request;
		request.start = $scope.start;
		request.end = $scope.end;
		angular.forEach($scope.query_p, function(p) {
			if (!p.metric) {
				return;
			}
			var q = new Query(p);
			var tags = q.tags;
			q.tags = new TagSet;
			angular.forEach(tags, function(v, k) {
				if (v && k) {
					q.tags[k] = v;
				}
			});
			request.queries.push(q);
		});
		return request;
	}
	$scope.Query = function() {
		var r = getRequest();
		r.prune();
		$location.search('b64', btoa(JSON.stringify(r)));
		$location.search('autods', $scope.autods ? undefined : 'false');
		$location.search('refresh', $scope.refresh ? 'true' : undefined);
		$route.reload();
	}
	request = getRequest();
	if (!request.queries.length) {
		return;
	}
	var autods = $scope.autods ? '&autods=' + $('#chart').width() : '';
	function get(noRunning: boolean) {
		$timeout.cancel(graphRefresh);
		if (!noRunning) {
			$scope.running = 'Running';
		}
		var autorate = '';
		for(var i = 0; i < request.queries.length; i++) {
			if (request.queries[i].derivative == 'auto') {
				autorate += '&autorate=' + i;
			}
		}
		$scope.animate();
		$http.get('/api/graph?' + 'b64=' + encodeURIComponent(btoa(JSON.stringify(request))) + autods + autorate)
			.success((data) => {
				$scope.result = data.Series;
				if (!$scope.result) {
					$scope.warning = 'No Results';
				} else {
					$scope.warning = '';
				}
				$scope.queries = data.Queries;
				$scope.running = '';
				$scope.error = '';
				var u = $location.absUrl();
				u = u.substr(0, u.indexOf('?')) + '?';
				u += 'b64=' + search.b64 + autods + autorate;
				$scope.url = u;
			})
			.error((error) => {
				$scope.error = error;
				$scope.running = '';
			})
			.finally(() => {
				$scope.stop();
				if ($scope.refresh) {
					graphRefresh = $timeout(() => { get(true); }, 5000);
				};
			});
	};
	get(false);
}]);

bosunApp.directive('tsPopup', () => {
	return {
		restrict: 'E',
		scope: {
			url: '=',
		},
		template: '<button class="btn btn-default" data-html="true" data-placement="bottom">embed</button>',
		link: (scope: any, elem: any, attrs: any) => {
			var button = $('button', elem);
			scope.$watch(attrs.url, (url: any) => {
				if (!url) {
					return;
				}
				var text = '<input type="text" onClick="this.select();" readonly="readonly" value="&lt;a href=&quot;' + url + '&quot;&gt;&lt;img src=&quot;' + url + '&.png=png&quot;&gt;&lt;/a&gt;">';
				button.popover({
					content: text,
				});
			});
		},
	};
});