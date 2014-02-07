/// <reference path="angular.d.ts" />
/// <reference path="angular-route.d.ts" />
/// <reference path="google.visualization.d.ts" />

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

/* For reference only: not hydrating yet.
class Alert {
	Name: string;
	Crit: string;
	Warn: string;
	Vars: {[key: string]: string};
}

class Schedule {
	Alerts: Alert[];
	Freq: number;
	Status: {[key: string]: Status};
}

class HistoryEvent {
	Status: string;
	Time: string;
}

class Status {
	Expr: string;
	Emailed: boolean;
	Group: {[key: string]: string};
	History: HistoryEvent[];
	Touched: string;

	Last() {
		return this.History[this.History.length-1];
	}
}
*/

interface IDashboardScope extends ng.IScope {
	schedule: any;
	last: (history: any[]) => any;
}

tsafControllers.controller('DashboardCtrl', ['$scope', '$http', function($scope: IDashboardScope, $http: ng.IHttpService) {
	$http.get('/api/alerts').success(data => {
		$scope.schedule = data;
	});
	$scope.last = (history: any[]) => {
		return history[history.length-1];
	}
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
	eval: () => void;
	error: string;
	running: string;
	result: any;
	json: (v: any) => string;
}

tsafControllers.controller('ExprCtrl', ['$scope', '$http', function($scope: IExprScope, $http: ng.IHttpService){
	$scope.expr = 'avg(q("avg:os.cpu{host=*}", "5m"))';
	$scope.eval = () => {
		$scope.error = '';
		$scope.running = $scope.expr;
		$scope.result = {};
		$http.get('/api/expr?q=' + encodeURIComponent($scope.expr))
			.success((data) => {
				$scope.result = data;
				$scope.running = '';
			})
			.error((error) => {
				$scope.error = error;
				$scope.running = '';
			});
	};
	$scope.json = (v: any) => {
		return JSON.stringify(v, null, '  ');
	};
}]);

interface TagSet {
	[tagk: string]: string
}

interface TagV {
	[tagk: string]: string[]
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
	rate: string;
	start: string;
	end: string;
	aggregators: string[];
	dsaggregators: string[];
	aggregator: string;
	GetTagKByMetric: () => void;
	MakeQuery: () => void;
	TagsAsQs: (ts: TagSet) => string;
	MakeParam: (k: string, v: string) => string;
	GetTagVs: (k: string) => void;
	result: any;
	dt: any;
}

tsafControllers.controller('GraphCtrl', ['$scope', '$http', function($scope: IGraphScope, $http: ng.IHttpService){
	//Might be better to get these from OpenTSDB's Aggregator API
	$scope.aggregators = ["sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.dsaggregators = ["", "sum", "min", "max", "avg", "dev", "zimsum", "mimmin", "minmax"];
	$scope.ds = "";
	$scope.aggregator = "sum";
	$scope.rate = "false"
	$scope.start = "1h-ago"
	//$scope.metric = "darwin.cpu.idle"
	$http.get('/api/metric')
		.success(function (data: string[]) {
			$scope.metrics = data;
		})
		.error(function (error) {
			$scope.error = 'Unable to fetch metrics: ' + error;
		});
	$scope.GetTagKByMetric = function() {
		$scope.tagset = {};
		$scope.tagvs = {};
		$http.get('/api/tagk/' + $scope.metric)
			.success(function (data: string[]) {
				if (data instanceof Array) {
					for (var i = 0; i < data.length; i++) {
						$scope.tagset[data[i]] = "";
						GetTagVs(data[i]);
					}
				}
			})
			.error(function (error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	var TagsAsQS = function(ts: TagSet) {
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
	var MakeParam = function(k: string, v: string) {
		if (v) {
			return encodeURIComponent(k) + "=" + encodeURIComponent(v) + "&";
		}
		return "";
	}
	var GetTagVs = function(k: string) {
		$http.get('/api/tagv/' + k + '/' + $scope.metric)
			.success(function (data: string[]) {
				$scope.tagvs[k] = data;
			})
			.error(function (error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	}
	$scope.MakeQuery = function() {
		var qs = "";
		qs += MakeParam("start", $scope.start);
		qs += MakeParam("end", $scope.end);
		qs += MakeParam("aggregator", $scope.aggregator);
		qs += MakeParam("metric", $scope.metric);
		qs += encodeURIComponent("rate") + "=" + encodeURIComponent($scope.rate) + "&";
		qs += MakeParam("tags", TagsAsQS($scope.tagset));
		if ($scope.ds && $scope.dstime) {
			qs += MakeParam("downsample", $scope.dstime + '-' + $scope.ds);
		}
		$scope.query = qs;
		$scope.running = $scope.query;
		$http.get('/api/query?' + $scope.query)
			.success((data) => {
				$scope.result = data.table;
				$scope.running = '';
			})
			.error((error) => {
				$scope.error = error;
				$scope.running = '';
			});

	}
}]);
 
tsafApp.directive("googleChart",function(){
    return {
        restrict : "A",
        link: function(scope: IGraphScope, elem: any, attrs: any) {
        	var chart : any;
        	var dt : any;
            chart = new google.visualization.LineChart(elem[0]);
            scope.$watch(attrs.ngModel, function(v: any, old_v: any) {
            	if (v != old_v) {
					dt = new google.visualization.DataTable(v);
					chart.draw(dt)
				}
	        });
        }
    }
});

