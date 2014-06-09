class Tag {
	k: string;
	v: string;
}

class DP {
	k: any;
	v: any;
}

interface IExprScope extends ng.IScope {
	metrics: string[];
	metric: string;
	tags: Tag[];
	dps: DP[];
}

tsafControllers.controller('PutCtrl', ['$scope', '$http', '$route', function($scope: IExprScope, $http: ng.IHttpService, $route: ng.route.IRouteService) {
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
	$scope.Submit = function() {
		var data: any = [];
		var tags: any = {};
		angular.forEach($scope.tags, function(v, k) {
			if (v.k && v.v) {
				tags[v.k] =  v.v;
			}
		});
		angular.forEach($scope.dps, function(v, k) {
			if (v.k && v.v) {
				var ts = moment.utc(v.k, mfmt).format("X");
				data.push({
					metric: $scope.metric,
					timestamp: ts,
					value: v.v,
					tags: tags,
				});
			}
		});
		console.log(JSON.stringify(data));
	}
	$scope.AddTag = function() {
		var last = $scope.tags[$scope.tags.length - 1];
		if (last.k && last.v) {
			$scope.tags.push(new Tag);
		}
	}
	$scope.AddDP = function() {
		var last = $scope.dps[$scope.dps.length - 1];
		if (last.k && last.v) {
			var dp = new DP;
			dp.k = moment.utc(last.k, mfmt).add("seconds", 15).format(mfmt);
			$scope.dps.push(dp);
		}
	}
	$scope.GetTagKByMetric = function() {
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