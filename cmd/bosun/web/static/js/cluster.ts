interface IClusterScope extends ng.IScope {
	error: string;
	warning: string;
	cluster: ClusterState;
	clusterPanelClass: any;
	promotePeer: any;
	loading: boolean;
}

bosunControllers.controller('ClusterCtrl', ['$scope', '$http', '$location', '$route', '$sce', 'linkService', function ($scope: IClusterScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService, $sce: ng.ISCEService, linkService: ILinkService) {
	$http.get('/api/cluster/status')
		.success((data: any) => {
			$scope.cluster = data;
		})
		.error((error) => {
			$scope.error = error;
		});
	$scope.clusterPanelClass = (state: string) => {
		switch (state) {
			case "Leader": return "panel-success";
			case "Follower": return "panel-info";
			case "Candidate": return "panel-warning";
			default: return "panel-default";
		}
	}

	$scope.promotePeer = (id: string, address: string) => {
		console.log("promote new peer", id)
		$scope.loading = true;
		$http.post('/api/cluster/change_master', {"address": address, "id": id})
		.success((data: any) => {
			if (data.status === "error") {
				$scope.error = data.error;
			} else {
				for (var i = 0; i < 10; i++) {
				$http.get('/api/cluster/status')
				.success((data: any) => {
					$scope.cluster = data;
				})
				.error((error) => {
					$scope.error = error;
				})
				.finally(() => {
					$scope.loading = false;
				})
				;
				}
			}
		})
		.error((error) => {
			$scope.error = error;
		});
	};
}]);

bosunApp.directive('tsPromote', () => {
	return {
		restrict: 'E',
		templateUrl: '/partials/promote.html',
	};
});
