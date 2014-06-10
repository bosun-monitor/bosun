class Tag {
	k: string;
	v: string;
}

class DP {
	k: any;
	v: any;
}

interface IPutScope extends ng.IScope {
	error: string;
	running: string;
	success: string;
	metrics: string[];
	metric: string;
	tags: Tag[];
	dps: DP[];
	Submit: () => void;
	GetTagKByMetric: () => void;
	AddTag: () => void;
	AddDP: () => void;
}

tsafControllers.controller('PutCtrl', ['$scope', '$http', '$route', function($scope: IPutScope, $http: ng.IHttpService, $route: ng.route.IRouteService) {
	var mfmt = 'YYYY/MM/DD-HH:mm:ss';
	$scope.tags = [new Tag];
	var dp = new DP;
	dp.k = moment().utc().format(mfmt);
	$scope.dps = [dp];
	$http.get('/api/metric')
		.success(function(data: string[]) {
			$scope.metrics = data;
		})
		.error(function(error) {
			$scope.error = 'Unable to fetch metrics: ' + error;
		});
	$scope.Submit = () => {
		var data: any = [];
		var tags: any = {};
		angular.forEach($scope.tags, (v, k) => {
			if (v.k || v.v) {
				if (!v.k) {
					$scope.error = 'Tag value ' + v.v + ' must have a key';
					return;
				}
				if (!v.v) {
					$scope.error = 'Tag key ' + v.k + ' must have a value';
					return;
				}
				tags[v.k] =  v.v;
			}
		});
		angular.forEach($scope.dps, (v, k) => {
			if (v.k && v.v) {
				var ts =  parseInt(moment.utc(v.k, mfmt).format('X'));
				data.push({
					metric: $scope.metric,
					timestamp: ts,
					value: parseFloat(v.v),
					tags: tags,
				});
			}
		});
		$scope.running = 'submitting data...';
		$scope.success = '';
		$scope.error = '';
		$http.post('/api/put', data)
			.success(() => {
				$scope.running = '';
				$scope.success = 'Data Submitted';
				})
			.error((error: any) => {
					$scope.running = '';
					$scope.error = error.error.message;
				});
	}
	$scope.AddTag = () => {
		var last = $scope.tags[$scope.tags.length - 1];
		if (last.k && last.v) {
			$scope.tags.push(new Tag);
		}
	}
	$scope.AddDP = () => {
		var last = $scope.dps[$scope.dps.length - 1];
		if (last.k && last.v) {
			var dp = new DP;
			dp.k = moment.utc(last.k, mfmt).add('seconds', 15).format(mfmt);
			$scope.dps.push(dp);
		}
	}
	$scope.GetTagKByMetric = () => {
		$http.get('/api/tagk/' + $scope.metric)
			.success(function(data: string[]) {
				if (!angular.isArray(data)) {
					return;
				}
				$scope.tags = [];
				for (var i = 0; i < data.length; i++) {
					var t = new Tag;
					t.k = data[i];
					$scope.tags.push(t);
				}
			})
			.error(function(error) {
				$scope.error = 'Unable to fetch metrics: ' + error;
			});
	};
}]);