tsafApp.directive('tsResults', function() {
	return {
		templateUrl: '/partials/results.html',
	};
});

var timeFormat = 'YYYY-MM-DD HH:mm:ss ZZ';

tsafApp.directive("tsTime", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTime, (v: any) => {
				var m = moment(v).utc();
				var el = document.createElement('a');
				el.innerText = m.format(timeFormat) +
				' (' +
				m.fromNow() +
				')';
				el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
				el.href += m.format('YYYYMMDDTHHmm');
				el.href += '&p1=0';
				angular.forEach(scope.timeanddate, (v, k) => {
					el.href += '&p' + (k + 2) + '=' + v;
				});
				elem.html(el);
			});
		},
	};
});

tsafApp.directive("tsTimeAndSince", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTimeAndSince, (v: any) => {
				var m = moment(v).utc();
				elem.text(m.format(timeFormat) + ' (' + m.fromNow() + ')');
			});
		},
	};
});

tsafApp.directive("tsSince", function() {
	return {
		link: function(scope: ITsafScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsSince, (v: any) => {
				var m = moment(v).utc();
				elem.text(m.fromNow());
			});
		},
	};
});

tsafApp.directive("tooltip", function() {
	return {
		link: function(scope: IGraphScope, elem: any, attrs: any) {
			angular.element(elem[0]).tooltip({ placement: "bottom" });
		},
	};
});

tsafApp.directive('tsLine', () => {
	return {
		link: (scope: any, elem: any, attrs: any) => {
			elem.linedtextarea();
			var parent = elem.parent();
			var linesDiv = parent
			function lineHighlight(line: any) {
				var lineHeight = elem[0].scrollHeight / (elem[0].value.match(/\n/g).length + 1);
				var jump = (line - 1) * lineHeight;
				elem.scrollTop(jump);
				elem.scroll();
				parent.find('.lines div').eq(line - 1).addClass('lineerror');
			}
			function lineClear() {
				parent.find('.lineerror').removeClass('lineerror');
			}
			scope.$watch(attrs.tsLine, (v: any) => {
				lineClear();
				if (v) {
					lineHighlight(v);
				}
			});
		},
	};
});

interface JQuery {
	tablesorter(v: any): JQuery;
}

tsafApp.directive('tsTableSort', ['$timeout', ($timeout: ng.ITimeoutService) => {
	return {
		link: (scope: ng.IScope, elem: any, attrs: any) => {
			$timeout(() => {
				$(elem).tablesorter({
					sortList: scope.$eval(attrs.tsTableSort),
				});
			});
		},
	};
}]);

tsafApp.directive('ahTimeLine', () => {
	//2014-05-26T21:46:37.435056942Z
	var format = d3.time.format.utc("%Y-%m-%dT%X");
	var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
	var parseDate = function(s: string) {
		return format.parse(s.split(".")[0]);
	}
	var margin = {
		top: 20,
		right: 80,
		bottom: 30,
		left: 80,
	};
	var customTimeFormat = d3.time.format.multi([
		[".%L", function(d: any) { return d.getMilliseconds(); }],
		[":%S", function(d: any) { return d.getSeconds(); }],
		["%I:%M", function(d: any) { return d.getMinutes(); }],
		["%H", function(d: any) { return d.getHours(); }],
		["%a %d", function(d: any) { return d.getDay() && d.getDate() != 1; }],
		["%b %d", function(d: any) { return d.getDate() != 1; }],
		["%B", function(d: any) { return d.getMonth(); }],
		["%Y", function() { return true; }]
	]);
	return {
		replace: false,
		scope: {
			data: '=',
		},
		link: (scope: any, elem: any, attrs: any) => {
			var svgHeight = elem.height();
			var height = svgHeight - margin.top - margin.bottom;
			var svgWidth = elem.width();
			var width = svgWidth - margin.left - margin.right;
			var xScale = d3.time.scale.utc().range([0, width]);
			var xAxis = d3.svg.axis()
				.scale(xScale)
				.tickFormat(customTimeFormat)
				.orient('bottom');
			var svg = d3.select(elem[0])
				.append('svg')
				.attr('width', svgWidth)
				.attr('height', svgHeight)
				.append('g')
				.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
			svg.append('defs')
				.append('clipPath')
				.attr('id', 'clip')
				.append('rect')
				.attr('width', width)
				.attr('height', height);
			var chart = svg.append('g')
				.attr('clip-path', 'url(#clip)');
			svg.append('g')
				.attr('class', 'x axis')
				.attr('transform', 'translate(0,' + height + ')');
			svg.append('g')
				.attr('class', 'y axis');
			var legend = d3.select('.legend')
				.append("p")
				.text(tsdbFormat(new Date));
			scope.$watch('data', update);
			function update(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
					return;
				}
				xScale.domain([
					d3.min(v, (d: any) => { return parseDate(d.Time); }),
					new Date(),
				]);
				svg.select('.x.axis')
					.transition()
					.call(xAxis);
				chart.selectAll(".bars")
					.data(v)
					.enter()
					.append("rect")
					.attr("class", (d: any) => { return d.Status; } )
					.attr("x", (d: any) => { return xScale(parseDate(d.Time)); })
					.attr("y", 0)
					.attr("height", height)
					.attr("width", (d: any, i: any) => {
						if (i+1 < v.length) {
							return xScale(parseDate(v[i+1].Time)) - xScale(parseDate(d.Time));
						}
						return xScale(new Date()) - xScale(parseDate(d.Time));
					})
					.on("mousemove", mousemove)
					.on("click", function(d) {
						var e = $("#" + 'a' + d.Time.replace( /(:|\.|\[|\])/g, "\\$1" ))
						e.click();
					});
				function mousemove() {
					var x: any = xScale.invert(d3.mouse(this)[0]);
					legend
						.text(tsdbFormat(x));
				}
			};
		},
	};
});

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s: any, mult: number, suffix: string, opts: any) {
	opts = opts || {};
	var n = parseFloat(s);
	if (opts.round) n = Math.round(n);
	if (!n) return suffix ? '0 ' + suffix : '0';
	if (isNaN(n) || !isFinite(n)) return '-';
	var a = Math.abs(n);
	var precision = a < 1 ? 2 : 4;
	if (a >= 1) {
		var number = Math.floor(Math.log(a) / Math.log(mult));
		a /= Math.pow(mult, Math.floor(number));
		if (fmtUnits[number]) {
			suffix = fmtUnits[number] + suffix;
		}
	}
	if (n < 0) a = -a;
	var r = a.toFixed(precision);
	return r + suffix;
}

tsafApp.filter('nfmt', function() {
	return function(s: any) {
		return nfmt(s, 1000, '', {});
	}
});

tsafApp.filter('bytes', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'B', { round: true });
	}
});

tsafApp.filter('bits', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'b', { round: true });
	}
});

tsafApp.filter('reverse', function() {
	return function(items: any) {
		if(typeof items === 'undefined') { return; }
		return angular.isArray(items) ?
			items.slice().reverse() : // If it is an array, split and reverse it
			(items + '').split('').reverse().join(''); // else make it a string (if it isn't already), and reverse it
	};
});
