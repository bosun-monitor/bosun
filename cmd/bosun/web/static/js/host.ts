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
	$scope.idata = [];
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
	$http.get('/api/tagv/iface/os.net.bytes?host=' + $scope.host)
		.success((data) => {
			angular.forEach(data, function(i, idx) {
				$scope.idata[idx] = {
					Name: i,
				};
			});
			function next(idx: number) {
				var idata = $scope.idata[idx];
				if (!idata || currentURL != $location.url()) {
					return;
				}
				var net_bytes_r = new Request();
				net_bytes_r.start = $scope.time;
				net_bytes_r.queries = [
					new Query({
						metric: "os.net.bytes",
						rate: true,
						rateOptions: { counter: true, resetValue: 1 },
						tags: { host: $scope.host, iface: idata.Name, direction: "*" },
					})
				];
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(net_bytes_r)) + autods)
					.success((data) => {
						if (!data.Series) {
							return;
						}
						angular.forEach(data.Series, function(d) {
							d.Data = d.Data.map((dp: any) => { return [dp[0], dp[1]*8]; });
							if (d.Name.indexOf("direction=out") != -1) {
								d.Data = d.Data.map((dp: any) => { return [dp[0],dp[1]* -1]; });
								d.Name = "out";
							} else {
								d.Name = "in";
							}
						});
						$scope.idata[idx].Data = data.Series;
					})
					.finally(() => {
						next(idx + 1);
					});
			}
			next(0);
		});
	$http.get('/api/tagv/disk/os.disk.fs.space_total?host=' + $scope.host)
		.success((data) => {
			angular.forEach(data, function(i, idx) {
				if (i == '/dev/shm') {
					return;
				}
				var fs_r = new Request();
				fs_r.start = $scope.time;
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_total",
					tags: { host: $scope.host, disk: i },
				}));
				fs_r.queries.push(new Query({
					metric: "os.disk.fs.space_used",
					tags: { host: $scope.host, disk: i },
				}));
				$scope.fsdata[idx] = {
					Name: i,
				};
				$http.get('/api/graph?' + 'json=' + encodeURIComponent(JSON.stringify(fs_r)) + autods)
					.success((data) => {
						if (!data.Series) {
							return;
						}
						data.Series[1].Name = 'Used';
						var total = Math.max.apply(null, data.Series[0].Data.map((d: any) => { return d[1]; }));
						var c_val = data.Series[1].Data.slice(-1)[0][1];
						var percent_used = c_val / total * 100;
						$scope.fsdata[idx].total = total;
						$scope.fsdata[idx].c_val = c_val;
						$scope.fsdata[idx].percent_used = percent_used;
						$scope.fsdata[idx].Data = [data.Series[1]];
					});
			});
		});
}]);
