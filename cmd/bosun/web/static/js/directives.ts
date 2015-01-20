bosunApp.directive('tsResults', function() {
	return {
		templateUrl: '/partials/results.html',
		link: (scope, elem, attrs) => {
			scope.isSeries = v => {
				return typeof(v) === 'object';
			};
		},
	};
});

bosunApp.directive('tsComputations', () => {
	return {
		scope: {
			computations: '=tsComputations',
			time: '=',
			header: '=',
		},
		templateUrl: '/partials/computations.html',
		link: (scope: any, elem: any, attrs: any) => {
			if (scope.time) {
				var m = moment.utc(scope.time);
				scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
			}
			scope.btoa = (v: any) => {
				return encodeURIComponent(btoa(v));
			};
		},
	};
});

function fmtDuration(v: any) {
	var diff = moment.duration(v, 'milliseconds');
	var f;
	if (Math.abs(v) < 60000) {
		return diff.format('ss[s]');
	}
	return diff.format('d[d]hh[h]mm[m]ss[s]');
}

function fmtTime(v: any) {
	var m = moment(v).utc();
	var now = moment().utc();
	var msdiff = now.diff(m);
	var ago = '';
	var inn = '';
	if (msdiff >= 0) {
		ago = ' ago';
	} else {
		inn = 'in ';
	}
	return m.format() + ' (' + inn + fmtDuration(msdiff) + ago + ')';
}

interface ITimeScope extends IBosunScope {
	noLink: string;
}

bosunApp.directive("tsTime", function() {
	return {
		link: function(scope: ITimeScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsTime, (v: any) => {
				var m = moment(v).utc();
				var text = fmtTime(v);
				if (attrs.tsEndTime) {
					var diff = moment(scope.$eval(attrs.tsEndTime)).diff(m);
					var duration = fmtDuration(diff);
					text += " for " + duration;
				}
				if (attrs.noLink) {
					elem.text(text);
				} else {
					var el = document.createElement('a');
					el.text = text;
					el.href = 'http://www.timeanddate.com/worldclock/converted.html?iso=';
					el.href += m.format('YYYYMMDDTHHmm');
					el.href += '&p1=0';
					angular.forEach(scope.timeanddate, (v, k) => {
						el.href += '&p' + (k + 2) + '=' + v;
					});
					elem.html(el);
				}
			});
		},
	};
});

bosunApp.directive("tsSince", function() {
	return {
		link: function(scope: IBosunScope, elem: any, attrs: any) {
			scope.$watch(attrs.tsSince, (v: any) => {
				var m = moment(v).utc();
				elem.text(m.fromNow());
			});
		},
	};
});

bosunApp.directive("tooltip", function() {
	return {
		link: function(scope: IGraphScope, elem: any, attrs: any) {
			angular.element(elem[0]).tooltip({ placement: "bottom" });
		},
	};
});

bosunApp.directive('tsLine', () => {
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

bosunApp.directive('tsTab', () => {
	return {
		link: (scope: any, elem: any, attrs: any) => {
			var ta = elem[0];
			elem.keydown(evt => {
				if (evt.ctrlKey) {
					return;
				}
				// This is so shift-enter can be caught to run a rule when tsTab is called from
				// the rule page
				if (evt.keyCode == 13 && evt.shiftKey) {
					return;
				}
				switch (evt.keyCode) {
				case 9: // tab
					evt.preventDefault();
					var v = ta.value;
					var start = ta.selectionStart;
					ta.value = v.substr(0, start) + "\t" + v.substr(start);
					ta.selectionStart = ta.selectionEnd = start + 1;
					return;
				case 13: // enter
					if (ta.selectionStart != ta.selectionEnd) {
						return;
					}
					evt.preventDefault();
					var v = ta.value;
					var start = ta.selectionStart;
					var sub = v.substr(0, start);
					var last = sub.lastIndexOf("\n") + 1
					for (var i = last; i < sub.length && /[ \t]/.test(sub[i]); i++)
						;
					var ws = sub.substr(last, i - last);
					ta.value = v.substr(0, start) + "\n" + ws + v.substr(start);
					ta.selectionStart = ta.selectionEnd = start + 1 + ws.length;
				}
			});
		},
	};
});

interface JQuery {
	tablesorter(v: any): JQuery;
}

bosunApp.directive('tsTableSort', ['$timeout', ($timeout: ng.ITimeoutService) => {
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

bosunApp.directive('tsHistory', () => {
	return {
		scope: {
			computations: '=tsComputations',
			time: '=',
			header: '=',
		},
		templateUrl: '/partials/history.html',
		link: (scope: any, elem: any, attrs: any) => {
			if (scope.time) {
				var m = moment.utc(scope.time);
				scope.timeParam = "&date=" + encodeURIComponent(m.format("YYYY-MM-DD")) + "&time=" + encodeURIComponent(m.format("HH:mm"));
			}
			scope.btoa = (v: any) => {
				return encodeURIComponent(btoa(v));
			};
		},
	}
});

bosunApp.directive('tsTimeLine', () => {
	var tsdbFormat = d3.time.format.utc("%Y/%m/%d-%X");
	function parseDate(s: any) {
		return moment.utc(s).toDate();
	}
    var margin = {
        top: 10,
        right: 10,
        bottom: 30,
        left: 250,
    };

    var font = {
        size: 10,
        weight: "bold",
        family: "sans-serif",
        variant: "small-caps",
        color: "black"
    }
    var plotOpts = {
        grid: {
            borderWidth: 0
        },
        xaxis: {
            font: font,
            mode: "time" ,
            // a sufficiently big enough number, so that multiple graphs align.
            // http://stackoverflow.com/questions/11149800/how-to-size-and-align-flot-graphs
            labelWidth: 100,
        },
        yaxis: {
            font: font,
        }
    };
    var dataOpts = {
        lines: { show: true, lineWidth: 1},
        points: { show: true, radius: 1 },
        color: "black"
    }

    var tagsetToId = function(expr) {
        return expr.replace(/([^a-z0-9]+)/gi, '-');
    }

	return {
		link: (scope: any, elem: any, attrs: any) => {
			scope.shown = {};
			scope.collapse = (i: any) => {
				scope.shown[i] = !scope.shown[i];
			};
			scope.$watch('alert_history', update);
			function update(history: any) {
				if (!history) {
					return;
				}
				var entries = d3.entries(history);
				if (!entries.length) {
					return;
				}
				entries.sort((a, b) => {
					return a.key.localeCompare(b.key);
				});
				scope.entries = entries;
				var values = entries.map(v => { return v.value });
				var keys = entries.map(v => { return v.key });
				var barheight = 500 / values.length;
				var barheight = Math.round(barheight);
				barheight = Math.min(barheight, 60);
				barheight = Math.max(barheight, 35);
				var svgHeight = values.length * barheight + margin.top + margin.bottom;
				var height = svgHeight - margin.top - margin.bottom;
				var svgWidth = elem.width();
				var width = svgWidth - margin.left - margin.right;
				elem.empty();
				var box = d3.select(elem[0]);

				angular.forEach(entries, function(entry: any, i: number) {
                    console.debug("entries entry:" + i);
                    console.debug(entry.value.History);
                    var statuses = {};
                    var first = -1;
                    var last = -1;
                    for (var i = 0; i < entry.value.History.length; i++) {
                    console.debug('iter ' + i);
                        var h = entry.value.History[i];
                        var start = moment.utc(h.Time).unix() * 1000
                        var end = moment.utc(h.EndTime).unix() * 1000
                        var status = statuses[h.Status] || []; // Status is normal/warning/critical
                        status.push({from:start, to:end-1}); // the -1 gives us a nice thin separator until the next block
                        statuses[h.Status] = status;
                        if (first == -1) {
                            first = start;
                        }
                        if (end > last) {
                            last = end;
                        }
                    }

                    box.append('span')
                        .attr('style', 'width:'+ width + 'px; height:' + barheight + 'px;')
                        .text(entry['key']);
                    // create divs into which we'll plot.
                    box.append('div')
                        .attr('style', 'width:'+ width + 'px; height:' + barheight + 'px;')
                        .attr('id', tagsetToId(entry['key']) + "_series_1");
                    box.append('div')
                        .attr('style', 'width:'+ width + 'px; height:' + barheight + 'px;')
                        .attr('id', tagsetToId(entry['key']) + "_status").text(".");

                    // test data
                    var data = [[first, 200000], [(first+last)/2, 600000], [last, 400000]];

                    // plot series
                    var plotSpec = $.extend(true, {}, plotOpts);
                    plotSpec['xaxis']['timeformat'] = "" // time will be shown in status graph, no need to show it on every graph
                    var dataSpec = $.extend({
                        data: data,
                    }, dataOpts);

                    $.plot('#' + tagsetToId(entry['key']) + "_series_1", [dataSpec], plotSpec);

                    // plot status
                    var markOpts = {
                        yaxis: {
                            show: false
                        },
                        grid: {
                            borderWidth: 0,
                            markings: []
                        }
                    };
                    var statusProps = {
                        normal: "green",
                        warning: "orange",
                        critical: "red"
                    };
                    for (var status in statuses) {
                        if (statuses.hasOwnProperty(status)) {
                            markOpts['grid']['markings'].push({ xaxis: statuses[status], color: statusProps[status]});
                        }
                    }
                    var plotSpec = $.extend({}, plotOpts, markOpts);
                    var dataSpec2 = {
                        lines: {show: false},
                        // we must plot at least 1 point or otherwise flot doesn't plot the chart
                        // and we must plot a point at the end or it doesn't look right.
                        data: [[first,0], [last,0]]
                    }
                    console.debug('dataSpec', dataSpec2);
                    console.debug('plotSpec', plotSpec);

                    $.plot('#' + tagsetToId(entry['key']) + "_status", [dataSpec2], plotSpec);
                });
			};
		},
	};
});

var fmtUnits = ['', 'k', 'M', 'G', 'T', 'P', 'E'];

function nfmt(s: any, mult: number, suffix: string, opts: any) {
	opts = opts || {};
	var n = parseFloat(s);
	if (isNaN(n) && typeof s === 'string') {
		return s;
	}
	if (opts.round) n = Math.round(n);
	if (!n) return suffix ? '0 ' + suffix : '0';
	if (isNaN(n) || !isFinite(n)) return '-';
	var a = Math.abs(n);
	if (a >= 1) {
		var number = Math.floor(Math.log(a) / Math.log(mult));
		a /= Math.pow(mult, Math.floor(number));
		if (fmtUnits[number]) {
			suffix = fmtUnits[number] + suffix;
		}
	}
	var r = a.toFixed(5);
	 if (a < 1e-5) {
		r = a.toString();
	}
	var neg = n < 0 ? '-' : '';
	return neg + (+r) + suffix;
}

bosunApp.filter('nfmt', function() {
	return function(s: any) {
		return nfmt(s, 1000, '', {});
	}
});

bosunApp.filter('bytes', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'B', { round: true });
	}
});

bosunApp.filter('bits', function() {
	return function(s: any) {
		return nfmt(s, 1024, 'b', { round: true });
	}
});

bosunApp.directive('tsGraph', ['$window', 'nfmtFilter', function($window: ng.IWindowService, fmtfilter: any) {
	var margin = {
		top: 10,
		right: 10,
		bottom: 30,
		left: 80,
	};
	return {
		scope: {
			data: '=',
			height: '=',
			generator: '=',
			brushStart: '=bstart',
			brushEnd: '=bend',
			enableBrush: '@',
			max: '=',
			min: '=',
		},
		link: (scope: any, elem: any, attrs: any) => {
			var svgHeight = +scope.height || 150;
			var height = svgHeight - margin.top - margin.bottom;
			var svgWidth: number;
			var width: number;
			var yScale = d3.scale.linear().range([height, 0]);
			var xScale = d3.time.scale.utc();
			var xAxis = d3.svg.axis()
				.orient('bottom');
			var yAxis = d3.svg.axis()
				.scale(yScale)
				.orient('left')
				.ticks(Math.min(10, height / 20))
				.tickFormat(fmtfilter);
			var line: any;
			switch (scope.generator) {
				case 'area':
					line = d3.svg.area();
					break;
				default:
					line = d3.svg.line();
			}
			var brush = d3.svg.brush()
				.x(xScale)
				.on('brush', brushed);
			line.y((d: any) => { return yScale(d[1]); });
			line.x((d: any) => { return xScale(d[0] * 1000); });
			var top = d3.select(elem[0])
				.append('svg')
				.attr('height', svgHeight)
				.attr('width', '100%');
			var svg = top
				.append('g')
				.attr('transform', 'translate(' + margin.left + ',' + margin.top + ')');
			var defs = svg.append('defs')
				.append('clipPath')
				.attr('id', 'clip')
				.append('rect')
				.attr('height', height);
			var chart = svg.append('g')
				.attr('pointer-events', 'all')
				.attr('clip-path', 'url(#clip)');
			svg.append('g')
				.attr('class', 'x axis')
				.attr('transform', 'translate(0,' + height + ')');
			svg.append('g')
				.attr('class', 'y axis');
			var paths = chart.append('g');
			chart.append('g')
				.attr('class', 'x brush');
			top.append('rect')
				.style('opacity', 0)
				.attr('x', 0)
				.attr('y', 0)
				.attr('height', height)
				.attr('width', margin.left)
				.style('cursor', 'pointer')
				.on('click', yaxisToggle);
			var legendTop = d3.select(elem[0]).append('div');
			var xloc = legendTop.append('div');
			xloc.style('float', 'left');
			var brushText = legendTop.append('div');
			brushText.style('float', 'right');
			var legend = d3.select(elem[0]).append('div');
			legend.style('clear', 'both');
			var color = d3.scale.ordinal().range([
				'#e41a1c',
				'#377eb8',
				'#4daf4a',
				'#984ea3',
				'#ff7f00',
				'#a65628',
				'#f781bf',
				'#999999',
			]);
			var mousex = 0;
			var mousey = 0;
			var oldx = 0;
			var hover = svg.append('g')
				.attr('class', 'hover')
				.style('pointer-events', 'none')
				.style('display', 'none');
			var hoverPoint = hover.append('svg:circle')
				.attr('r', 5);
			var hoverRect = hover.append('svg:rect')
				.attr('fill', 'white');
			var hoverText = hover.append('svg:text')
				.style('font-size', '12px');
			var focus = svg.append('g')
				.attr('class', 'focus')
				.style('pointer-events', 'none');
			focus.append('line');
			function mousemove() {
				var pt = d3.mouse(this);
				mousex = pt[0];
				mousey = pt[1];
				if (scope.data) {
					drawLegend();
				}
			}
			var yaxisZero = false;
			function yaxisToggle() {
				yaxisZero = !yaxisZero;
				draw();
			}
			var drawLegend = _.throttle(() => {
				var names = legend.selectAll('.series')
					.data(scope.data, (d) => { return d.Name; });
				names.enter()
					.append('div')
					.attr('class', 'series');
				names.exit()
					.remove();
				var xi = xScale.invert(mousex);
				xloc.text('Time: ' + fmtTime(xi));
				var t = xi.getTime() / 1000;
				var minDist = width + height;
				var minName: string, minColor: string;
				var minX: number, minY: number;
				names
					.each(function(d: any) {
						var idx = bisect(d.Data, t);
						if (idx >= d.Data.length) {
							idx = d.Data.length - 1;
						}
						var e = d3.select(this);
						var pt = d.Data[idx];
						if (pt) {
							e.attr('title', pt[1]);
							e.text(d.Name + ': ' + fmtfilter(pt[1]));
							var ptx = xScale(pt[0] * 1000);
							var pty = yScale(pt[1]);
							var ptd = Math.sqrt(
								Math.pow(ptx - mousex, 2) +
								Math.pow(pty - mousey, 2)
							);
							if (ptd < minDist) {
								minDist = ptd;
								minX = ptx;
								minY = pty;
								minName = d.Name + ': ' + pt[1];
								minColor = color(d.Name);
							}
						}
					})
					.style('color', (d: any) => { return color(d.Name); });
				hover
					.attr('transform', 'translate(' + minX + ',' + minY + ')');
				hoverPoint.style('fill', minColor);
				hoverText
					.text(minName)
					.style('fill', minColor);
				var isRight = minX > width / 2;
				var isBottom = minY > height / 2;
				hoverText
					.attr('x', isRight ? -5 : 5)
					.attr('y', isBottom ? -8 : 15)
					.attr('text-anchor', isRight ? 'end' : 'start');
				var node: any = hoverText.node();
				var bb = node.getBBox();
				hoverRect
					.attr('x', bb.x - 1)
					.attr('y', bb.y - 1)
					.attr('height', bb.height + 2)
					.attr('width', bb.width + 2);
				var x = mousex;
				if (x > width) {
					x = 0;
				}
				focus.select('line')
					.attr('x1', x)
					.attr('x2', x)
					.attr('y1', 0)
					.attr('y2', height);
				if (extentStart) {
					var s = extentStart;
					if (extentEnd != extentStart) {
						s += ' - ' + extentEnd;
						s += ' (' + extentDiff + ')'
					}
					brushText.text(s);
				}
			}, 50);
			scope.$watch('data', update);
			var w = angular.element($window);
			scope.$watch(() => {
				return w.width();
			}, resize, true);
			w.bind('resize', () => {
				scope.$apply();
			});
			function resize() {
				svgWidth = elem.width();
				if (svgWidth <= 0) {
					return;
				}
				width = svgWidth - margin.left - margin.right;
				xScale.range([0, width]);
				xAxis.scale(xScale);
				if (!mousex) {
					mousex = width + 1;
				}
				svg.attr('width', svgWidth);
				defs.attr('width', width);
				xAxis.ticks(width / 60);
				draw();
			}
			var oldx = 0;
			var bisect = d3.bisector((d) => { return d[0]; }).left;
			function update(v: any) {
				if (!angular.isArray(v) || v.length == 0) {
					return;
				}
				resize();
			}
			function draw() {
				if (!scope.data) {
					return;
				}
				var xdomain = [
					d3.min(scope.data, (d: any) => { return d3.min(d.Data, (c: any) => { return c[0]; }); }) * 1000,
					d3.max(scope.data, (d: any) => { return d3.max(d.Data, (c: any) => { return c[0]; }); }) * 1000,
				];
				if (!oldx) {
					oldx = xdomain[1];
				}
				xScale.domain(xdomain);
				var ymin = d3.min(scope.data, (d: any) => { return d3.min(d.Data, (c: any) => { return c[1]; }); });
				var ymax = d3.max(scope.data, (d: any) => { return d3.max(d.Data, (c: any) => { return c[1]; }); });
				var diff = (ymax - ymin) / 50;
				if (!diff) {
					diff = 1;
				}
				ymin -= diff;
				ymax += diff;
				if (yaxisZero) {
					if (ymin > 0) {
						ymin = 0;
					} else if (ymax < 0) {
						ymax = 0;
					}
				}
				var ydomain = [ymin, ymax];
				if (angular.isNumber(scope.min)) {
					ydomain[0] = +scope.min;
				}
				if (angular.isNumber(scope.max)) {
					ydomain[1] = +scope.max;
				}
				yScale.domain(ydomain);
				if (scope.generator == 'area') {
					line.y0(yScale(0));
				}
				svg.select('.x.axis')
					.transition()
					.call(xAxis);
				svg.select('.y.axis')
					.transition()
					.call(yAxis);
				svg.append('text')
					.attr("class", "ylabel")
					.attr("transform", "rotate(-90)")
					.attr("y", -margin.left)
					.attr("x", - (height / 2))
					.attr("dy", "1em")
					.text(_.uniq(scope.data.map(v => { return v.Unit })).join("; "));
				var queries = paths.selectAll('.line')
					.data(scope.data, (d) => { return d.Name; });
				switch (scope.generator) {
					case 'area':
						queries.enter()
							.append('path')
							.attr('stroke', (d: any) => { return color(d.Name); })
							.attr('class', 'line')
							.style('fill', (d: any) => { return color(d.Name); });
						break;
					default:
						queries.enter()
							.append('path')
							.attr('stroke', (d: any) => { return color(d.Name); })
							.attr('class', 'line');
				}
				queries.exit()
					.remove();
				queries
					.attr('d', (d: any) => { return line(d.Data); })
					.attr('transform', null)
					.transition()
					.ease('linear')
					.attr('transform', 'translate(' + (xScale(oldx) - xScale(xdomain[1])) + ')');
				chart.select('.x.brush')
					.call(brush)
					.selectAll('rect')
					.attr('height', height)
					.on('mouseover', () => {
						hover.style('display', 'block');
					})
					.on('mouseout', () => {
						hover.style('display', 'none');
					})
					.on('mousemove', mousemove);
				chart.select('.x.brush .extent')
					.style('stroke', '#fff')
					.style('fill-opacity', '.125')
					.style('shape-rendering', 'crispEdges');
				oldx = xdomain[1];
				drawLegend();
			};
			var extentStart: string;
			var extentEnd: string;
			var extentDiff: string;
			function brushed() {
				var extent = brush.extent();
				extentStart = datefmt(extent[0]);
				extentEnd = datefmt(extent[1]);
				extentDiff = fmtDuration(moment(extent[1]).diff(moment(extent[0])));
				drawLegend();
				if (scope.enableBrush && extentEnd != extentStart) {
					scope.brushStart = extentStart;
					scope.brushEnd = extentEnd;
					scope.$apply();
				}
			}
			var mfmt = 'YYYY/MM/DD-HH:mm:ss';
			function datefmt(d: any) {
				return moment(d).utc().format(mfmt);
			}
		},
	};
}]);
