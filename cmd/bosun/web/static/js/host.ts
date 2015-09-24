interface IHostScope extends ng.IScope {
	cpu: any;
	host: string;
	time: string;
	tab: string;
	metrics: string[];
	mlink: (m: string) => Request;
	setTab: (m: string) => void;
	idata: any;
	fsdata: any;
	mem: any;
	mem_total: number;
	error: string;
	running: string;
	filterMetrics: string;
	metadata: any;
}

bosunControllers.controller('HostCtrl', ['$scope', '$http', '$location', '$route', function($scope: IHostScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	var search = $location.search();
	$scope.host = search.host;
	$scope.time = search.time;
	$scope.tab = search.tab || "stats";
	$scope.fsdata = [];
	$scope.metrics = [];
	var currentURL = $location.url();
	$scope.mlink = (m: string) => {
		var r = new Request();
		var q = new Query();
		q.metric = m;
		q.tags = { 'host': $scope.host };
		r.queries.push(q);
		return r;
	};
	$scope.setTab = function(t: string) {
		$location.search('tab', t);
		$scope.tab = t;
	};
	$http.get('/api/metric/host/' + $scope.host)
		.success(function(data: string[]) {
			$scope.metrics = data || [];
		});
	var start = moment().utc().subtract(parseDuration($scope.time));
	function parseDuration(v: string) {
		var pattern = /(\d+)(d|y|n|h|m|s)-ago/;
		var m = pattern.exec(v);
		return moment.duration(parseInt(m[1]), m[2].replace('n', 'M'))
	}
	$http.get('/api/metadata/get?tagk=host&tagv=' + encodeURIComponent($scope.host))
		.success((data) => {
			$scope.metadata = _.filter(data, function(i: any) {
				return moment.utc(i.Time) > start;
			});
		});
	var autods = '&autods=100';
	var cpu_r = new Request();
	cpu_r.start = $scope.time;
	cpu_r.queries = [
		new Query({
			metric: 'os.cpu',
			derivative: 'counter',
			tags: { host: $scope.host },
		})
	];
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(cpu_r)) + autods)
		.success((data) => {
			if (!data.Series) {
				return;
			}
			data.Series[0].Name = 'Percent Used';
			$scope.cpu = data.Series;
		});
	var mem_r = new Request();
	mem_r.start = $scope.time;
	mem_r.queries.push(new Query({
		metric: "os.mem.total",
		tags: { host: $scope.host },
	}));
	mem_r.queries.push(new Query({
		metric: "os.mem.used",
		tags: { host: $scope.host },
	}));
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(mem_r)) + autods)
		.success((data) => {
			if (!data.Series) {
				return;
			}
			data.Series[1].Name = "Used";
			$scope.mem_total = Math.max.apply(null, data.Series[0].Data.map((d: any) => { return d[1]; }));
			$scope.mem = [data.Series[1]];
		});
	var net_bytes_r = new Request();
	net_bytes_r.start = $scope.time;
	net_bytes_r.queries = [
		new Query({
			metric: "os.net.bytes",
			rate: true,
			rateOptions: { counter: true, resetValue: 1 },
			tags: { host: $scope.host, iface: "*", direction: "*" },
		})
	];
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + autods)
		.success((data) => {
			if (!data.Series) {
					return;
			}
			var tmp = [];
			var ifaceSeries = {};
			angular.forEach(data.Series, function(series, idx) {
					series.Data = series.Data.map((dp: any) => { return [dp[0], dp[1] * 8]; });
					if (series.Tags.direction == "out") {
						 series.Data = series.Data.map((dp: any) => { return [dp[0], dp[1] * -1]; });
					}
					if (!ifaceSeries.hasOwnProperty(series.Tags.iface)) {
						 ifaceSeries[series.Tags.iface] = [series];
					} else {
						 ifaceSeries[series.Tags.iface].push(series);
						 tmp.push(ifaceSeries[series.Tags.iface]);
					}
			});
			$scope.idata = tmp;
		});
	var fs_r = new Request();
	fs_r.start = $scope.time
	fs_r.queries = [
		new Query({
			metric: "os.disk.fs.space_total",
			tags: { host: $scope.host, disk: "*"},
		}),
		new Query({
			metric: "os.disk.fs.space_used",
			tags: { host: $scope.host, disk: "*"},
		})
	];
	$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + autods)
		.success((data) => {
			if (!data.Series) {
					return;
			}
			var tmp = [];
			var fsSeries = {};
			angular.forEach(data.Series, function(series, idx) {
				var stat = series.Data[series.Data.length-1][1];
				var prop = "";
				if (series.Metric == "os.disk.fs.space_total") {
					prop = "total";
				} else {
					prop = "used";
				}
				if (!fsSeries.hasOwnProperty(series.Tags.disk)) {
					 fsSeries[series.Tags.disk] = [series];
					 fsSeries[series.Tags.disk][prop] = stat;
				} else {
					 fsSeries[series.Tags.disk].push(series);
					 fsSeries[series.Tags.disk][prop] = stat;
					 tmp.push(fsSeries[series.Tags.disk]);
				}
			});
			$scope.fsdata = tmp;
		});
}]);
