interface IErrorScope extends IBosunScope {
	errors: any;
	error: string;
	loading: boolean;
	totalLines: () => number;
	selectedLines: ()=> number;
	check: (err: any) => void;
	click: (err: any, event:any) => void;
	clearAll: () => void;
	clearSelected: () => void;
	ruleLink: (line:any, err:any) => string;
}

bosunControllers.controller('ErrorCtrl', ['$scope', '$http', '$location', '$route', function($scope: IErrorScope, $http: ng.IHttpService, $location: ng.ILocationService, $route: ng.route.IRouteService) {
	$scope.loading = true
	$http.get('/api/errors')
		.success((data) => {
			$scope.errors = [];
			_(data).forEach((err,name)=>{
				err.Name = name;
				err.Sum = 0;
				err.Shown = true;
				_(err.Errors).forEach((line)=>{
					err.Sum += line.Count
					line.FirstTime = moment.utc(line.FirstTime)
					line.LastTime = moment.utc(line.LastTime)
				})
				$scope.errors.push(err);
			})
		})
		.error(function(data) {
   			$scope.error = "Error fetching data: " + data;
  		})
		.finally(()=>{$scope.loading=false})
	
	$scope.check = (err) => {
		if (err.checked && !err.Shown){
			err.Shown = true;
		}
		_(err.Errors).forEach((line)=>{
			line.checked = err.checked;
		})
	};
	
	$scope.click = (err, event) => {
		event.stopPropagation();
	};
	
	$scope.totalLines = () => {
		var t = 0;
		_($scope.errors).forEach((err) =>{
			t += err.Errors.length;
		})
		return t;
	};
	
	$scope.selectedLines = () => {
		var t = 0;
		_($scope.errors).forEach((err) =>{
			_(err.Errors).forEach((line)=>{
				if(line.checked){
					t++;
				}
			})
		})
		return t;
	};
	
	var getKeys = (checkedOnly: boolean) => {
		var keys = [];
		_($scope.errors).forEach((err) =>{
			_(err.Errors).forEach((line)=>{
				if(!checkedOnly || line.checked){
					keys.push({alert:err.Name, start: line.FirstTime})
				}
			})
		})
		return keys;
	}
	
	var clear = (keys) => {
		$http.post('/api/errors', keys)
		.success((data) => {
			$route.reload();
		})
		.error(function(data) {
   			$scope.error = "Error Clearing Errors: " + data;
  		})
	}
	
	$scope.clearAll = () =>{
		var keys = getKeys(false);
		clear(keys);
	}
	
	$scope.clearSelected = () => {
		var keys = getKeys(true);
		clear(keys);
	}
	
	$scope.ruleLink = (line,err) => {
		var url = "/config?alert=" + err.Name;
		var fromDate = moment.utc(line.FirstTime)
		url += "&fromDate=" + fromDate.format("YYYY-MM-DD");
		url += "&fromTime=" + fromDate.format("hh:mm")
		var toDate = moment.utc(line.LastTime)
		url += "&toDate=" + toDate.format("YYYY-MM-DD");
		url += "&toTime=" + toDate.format("hh:mm")
		return url;
	}
}]);